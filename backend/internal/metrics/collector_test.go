package metrics

import (
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

type fakeSrc struct {
	st quic.ConnectionStats
}

func (f fakeSrc) ConnectionStats() quic.ConnectionStats { return f.st }
func (f fakeSrc) ConnectionState() quic.ConnectionState { return quic.ConnectionState{} }

func TestObserveRates(t *testing.T) {
	c := NewCollector()
	c.Observe(fakeSrc{st: quic.ConnectionStats{BytesSent: 1000, PacketsSent: 10}})
	time.Sleep(20 * time.Millisecond)
	_, _, send, _, _ := c.Observe(fakeSrc{st: quic.ConnectionStats{BytesSent: 5000, PacketsSent: 20, PacketsLost: 2}})
	if send == 0 {
		t.Fatal("expected positive send bps")
	}
}

func TestFPSRing(t *testing.T) {
	c := NewCollector()
	for i := 0; i < 5; i++ {
		c.NoteFrame("video")
	}
	var s Snapshot
	c.Fill(&s)
	if s.VideoFPS < 5 {
		t.Fatalf("fps=%v", s.VideoFPS)
	}
}

func TestNonMonotonicLoss(t *testing.T) {
	c := NewCollector()
	c.Observe(fakeSrc{st: quic.ConnectionStats{PacketsSent: 10, PacketsLost: 5}})
	time.Sleep(5 * time.Millisecond)
	_, _, _, _, loss := c.Observe(fakeSrc{st: quic.ConnectionStats{PacketsSent: 12, PacketsLost: 3}})
	if loss != 0 {
		t.Fatalf("loss should clamp when decreasing, got %v", loss)
	}
}
