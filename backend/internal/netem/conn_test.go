package netem

import (
	"bytes"
	"net"
	"testing"
	"time"
)

type loopback struct {
	ch     chan pkt
	closed chan struct{}
}

type pkt struct {
	b    []byte
	addr net.Addr
}

func newLoop() *loopback {
	return &loopback{ch: make(chan pkt, 64), closed: make(chan struct{})}
}

func (l *loopback) ReadFrom(p []byte) (int, net.Addr, error) {
	select {
	case <-l.closed:
		return 0, nil, net.ErrClosed
	case pk := <-l.ch:
		n := copy(p, pk.b)
		return n, pk.addr, nil
	}
}

func (l *loopback) WriteTo(p []byte, addr net.Addr) (int, error) {
	select {
	case <-l.closed:
		return 0, net.ErrClosed
	case l.ch <- pkt{b: append([]byte(nil), p...), addr: addr}:
		return len(p), nil
	default:
		return len(p), nil
	}
}

func (l *loopback) Close() error {
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
	return nil
}

func (l *loopback) LocalAddr() net.Addr                { return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9} }
func (l *loopback) SetDeadline(time.Time) error        { return nil }
func (l *loopback) SetReadDeadline(time.Time) error    { return nil }
func (l *loopback) SetWriteDeadline(time.Time) error   { return nil }

func TestDropAll(t *testing.T) {
	d := NewDual(1)
	_ = d.ApplyPreset("50")
	d.Apply(Profile{Name: "100", LossPct: 100}, Profile{Name: "100", LossPct: 100})
	c := Wrap(newLoop(), d)
	n, err := c.WriteTo([]byte("x"), c.LocalAddr())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if c.Stats().WriteDrops != 1 {
		t.Fatalf("drops=%d", c.Stats().WriteDrops)
	}
}

func TestPassthroughZero(t *testing.T) {
	d := NewDual(7)
	inner := newLoop()
	c := Wrap(inner, d)
	if _, err := c.WriteTo([]byte("abc"), c.LocalAddr()); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	n, _, err := c.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf[:n], []byte("abc")) {
		t.Fatalf("got %q", buf[:n])
	}
}

func TestPresetReproducibleDropRate(t *testing.T) {
	count := func(seed int64) int {
		d := NewDual(seed)
		d.Apply(Profile{LossPct: 30}, Profile{})
		c := Wrap(newLoop(), d)
		drops := 0
		for i := 0; i < 200; i++ {
			_, _ = c.WriteTo([]byte{byte(i)}, c.LocalAddr())
		}
		drops = int(c.Stats().WriteDrops)
		return drops
	}
	a, b := count(99), count(99)
	if a != b {
		t.Fatalf("same seed diverged %d vs %d", a, b)
	}
	if a < 20 || a > 100 {
		t.Fatalf("30%% of 200 should be ~60, got %d", a)
	}
}

func TestUnknownPreset(t *testing.T) {
	if _, err := Preset("99"); err == nil {
		t.Fatal("expected error")
	}
}
