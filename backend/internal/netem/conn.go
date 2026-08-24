package netem

import (
	"net"
	"sync/atomic"
	"time"
)

// Conn wraps a net.PacketConn and applies Dual profiles at UDP-packet
// granularity. This is a real impairment — QUIC loss detection, ACK and
// Cubic will react. It is NOT a mock of application metrics.
type Conn struct {
	inner   net.PacketConn
	dual    *Dual
	upRNG   *RNG
	downRNG *RNG

	readDrops  atomic.Uint64
	writeDrops atomic.Uint64
	readDelay  atomic.Uint64
	writeDelay atomic.Uint64

	closed atomic.Bool
}

func Wrap(inner net.PacketConn, dual *Dual) *Conn {
	seed := dual.Seed()
	return &Conn{
		inner:   inner,
		dual:    dual,
		upRNG:   NewRNG(seed),
		downRNG: NewRNG(seed ^ 0x5bd1e995),
	}
}

func (c *Conn) Dual() *Dual { return c.dual }

func (c *Conn) Stats() ConnStats {
	return ConnStats{
		ReadDrops:  c.readDrops.Load(),
		WriteDrops: c.writeDrops.Load(),
		ReadDelay:  c.readDelay.Load(),
		WriteDelay: c.writeDelay.Load(),
	}
}

type ConnStats struct {
	ReadDrops  uint64 `json:"read_drops"`
	WriteDrops uint64 `json:"write_drops"`
	ReadDelay  uint64 `json:"read_delayed"`
	WriteDelay uint64 `json:"write_delayed"`
}

func (c *Conn) ReadFrom(p []byte) (int, net.Addr, error) {
	for {
		if c.closed.Load() {
			return 0, nil, net.ErrClosed
		}
		n, addr, err := c.inner.ReadFrom(p)
		if err != nil {
			return n, addr, err
		}
		prof := c.dual.Downlink()
		if c.downRNG.Drop(prof.LossPct) {
			c.readDrops.Add(1)
			continue
		}
		if wait := c.downRNG.Jitter(prof.DelayMs, prof.JitterMs); wait > 0 {
			c.readDelay.Add(1)
			timer := time.NewTimer(time.Duration(wait) * time.Millisecond)
			<-timer.C
		}
		return n, addr, nil
	}
}

func (c *Conn) WriteTo(p []byte, addr net.Addr) (int, error) {
	if c.closed.Load() {
		return 0, net.ErrClosed
	}
	prof := c.dual.Uplink()
	if c.upRNG.Drop(prof.LossPct) {
		c.writeDrops.Add(1)
		return len(p), nil // silent drop, UDP semantics
	}
	wait := c.upRNG.Jitter(prof.DelayMs, prof.JitterMs)
	if c.upRNG.Drop(prof.ReorderPct) {
		wait += prof.ReorderDelay
	}
	if wait <= 0 {
		return c.inner.WriteTo(p, addr)
	}
	c.writeDelay.Add(1)
	// Copy the packet bytes so the delayed write is immune to the
	// caller reusing or overwriting its buffer after we return. A bare
	// slice expression (p[:len(p):len(p)]) would still alias the
	// caller's backing array, causing cross-packet corruption when the
	// buffer is reused while the write is queued.
	cp := make([]byte, len(p))
	copy(cp, p)
	go func() {
		time.Sleep(time.Duration(wait) * time.Millisecond)
		if c.closed.Load() {
			return
		}
		_, _ = c.inner.WriteTo(cp, addr)
	}()
	return len(p), nil
}

func (c *Conn) Close() error {
	c.closed.Store(true)
	return c.inner.Close()
}

func (c *Conn) LocalAddr() net.Addr                { return c.inner.LocalAddr() }
func (c *Conn) SetDeadline(t time.Time) error      { return c.inner.SetDeadline(t) }
func (c *Conn) SetReadDeadline(t time.Time) error  { return c.inner.SetReadDeadline(t) }
func (c *Conn) SetWriteDeadline(t time.Time) error { return c.inner.SetWriteDeadline(t) }
