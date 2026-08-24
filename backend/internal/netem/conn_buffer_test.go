package netem_test

import (
	"bytes"
	"net"
	"testing"
	"time"

	"golive/internal/netem"
)

type capturePacketConn struct {
	written chan []byte
}

func (c *capturePacketConn) ReadFrom([]byte) (int, net.Addr, error) {
	return 0, nil, net.ErrClosed
}

func (c *capturePacketConn) WriteTo(p []byte, _ net.Addr) (int, error) {
	c.written <- append([]byte(nil), p...)
	return len(p), nil
}

func (c *capturePacketConn) Close() error                     { return nil }
func (c *capturePacketConn) LocalAddr() net.Addr              { return &net.UDPAddr{} }
func (c *capturePacketConn) SetDeadline(time.Time) error      { return nil }
func (c *capturePacketConn) SetReadDeadline(time.Time) error  { return nil }
func (c *capturePacketConn) SetWriteDeadline(time.Time) error { return nil }

func TestDelayedWritePreservesDatagram(t *testing.T) {
	sender := &capturePacketConn{written: make(chan []byte, 1)}
	dual := netem.NewDual(17)
	dual.Apply(netem.Profile{DelayMs: 20}, netem.Profile{})
	conn := netem.Wrap(sender, dual)
	defer conn.Close()

	payload := bytes.Repeat([]byte{0x2a}, 16)
	want := append([]byte(nil), payload...)
	if n, err := conn.WriteTo(payload, sender.LocalAddr()); err != nil || n != len(payload) {
		t.Fatalf("WriteTo() = (%d, %v), want (%d, nil)", n, err, len(payload))
	}
	for i := range payload {
		payload[i] = 0xd4
	}

	select {
	case got := <-sender.written:
		if !bytes.Equal(got, want) {
			t.Fatalf("received datagram = %x, want original bytes %x", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for delayed datagram")
	}
}
