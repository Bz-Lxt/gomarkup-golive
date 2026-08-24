package bbr

import (
	"testing"
	"time"

	"golive/internal/config"
)

func testCfg() config.BBRParams {
	return config.BBRParams{
		Enabled:          true,
		ProbeRTTInterval: 200 * time.Millisecond,
		ProbeRTTDuration: 20 * time.Millisecond,
		StartupGain:      2.885,
		DrainGain:        0.346,
		BtlBwWindow:      8,
		MinPacingBps:     8 * 1024,
		MaxPacingBps:     40 * 1024 * 1024,
	}
}

func TestStartupThenDrainThenProbeBW(t *testing.T) {
	c := New(testCfg())
	now := time.Now().UnixNano()
	var sent, recv uint64
	var last State
	transitions := 0
	for i := 0; i < 80; i++ {
		now += int64(20 * time.Millisecond)
		sent += 50_000
		recv += 48_000
		out := c.OnSample(Sample{
			MinRTT: int64(12 * time.Millisecond), SmoothedRTT: int64(15 * time.Millisecond),
			BytesSent: sent, BytesReceived: recv, NowNs: now,
		})
		if i == 0 {
			last = out.State
			continue
		}
		if out.State != last {
			transitions++
			last = out.State
		}
	}
	if c.Snapshot().State == Startup && transitions == 0 {
		t.Fatal("never left Startup under growing delivery")
	}
	if c.Snapshot().PacingBps < testCfg().MinPacingBps {
		t.Fatal("pacing below floor")
	}
}

func TestDisabledOpensPacer(t *testing.T) {
	cfg := testCfg()
	cfg.Enabled = false
	c := New(cfg)
	out := c.OnSample(Sample{NowNs: 1, BytesSent: 100})
	if out.PacingBps != cfg.MaxPacingBps {
		t.Fatalf("disabled pacing=%d", out.PacingBps)
	}
}

func TestProbeRTTEnter(t *testing.T) {
	cfg := testCfg()
	cfg.ProbeRTTInterval = 30 * time.Millisecond
	c := New(cfg)
	now := time.Now().UnixNano()
	var sent uint64
	saw := false
	for i := 0; i < 40; i++ {
		now += int64(10 * time.Millisecond)
		sent += 20_000
		out := c.OnSample(Sample{
			MinRTT: int64(8 * time.Millisecond), SmoothedRTT: int64(10 * time.Millisecond),
			BytesSent: sent, BytesReceived: sent, NowNs: now,
		})
		if out.State == ProbeRTT {
			saw = true
			if out.CwndBytes != 4*1200 {
				t.Fatalf("probeRTT cwnd=%d", out.CwndBytes)
			}
			break
		}
	}
	if !saw {
		t.Fatal("expected ProbeRTT")
	}
}

func TestMaxFilter(t *testing.T) {
	f := newMaxFilter(3)
	f.Add(1)
	f.Add(5)
	f.Add(3)
	if f.Max() != 5 {
		t.Fatal(f.Max())
	}
	f.Add(2)
	f.Add(2)
	f.Add(2)
	if f.Max() != 2 {
		t.Fatalf("window expired, got %d", f.Max())
	}
}
