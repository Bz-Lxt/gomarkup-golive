// Package bbr implements BBR v1 as an application-layer rate controller.
//
// Inputs: real quic.ConnectionStats samples.
// Outputs: pacing_rate and cwnd that drive the scheduler token bucket.
//
// This is NOT a replacement of quic-go's internal Cubic. quic-go v0.61.0
// does not expose a congestion-control hook (see docs/.meta/api_contracts.md).
package bbr

import (
	"sync"
	"time"

	"golive/internal/config"
)

var probeBWCycle = []float64{1.25, 0.75, 1, 1, 1, 1, 1, 1}

const (
	defaultPacket = 1200
	minCwndPkts   = 4
	startupPlateau = 3
)

type Controller struct {
	mu sync.Mutex

	cfg config.BBRParams
	st  State

	btlbw  *maxFilter
	rtprop *minFilter

	pacingGain float64
	cwndGain   float64
	cycle      int
	filled     bool
	rounds     uint64
	fullBw     uint64
	fullBwCnt  int

	lastSent uint64
	lastRecv uint64
	lastNs   int64

	probeRTTDue  int64
	probeRTTEnd  int64
	priorCwnd    uint64
	priorState   State

	out Output
}

func New(cfg config.BBRParams) *Controller {
	c := &Controller{
		cfg:        cfg,
		st:         Startup,
		btlbw:      newMaxFilter(cfg.BtlBwWindow),
		rtprop:     newMinFilter(max(8, cfg.BtlBwWindow)),
		pacingGain: cfg.StartupGain,
		cwndGain:   cfg.StartupGain,
		probeRTTDue: time.Now().UnixNano() + int64(cfg.ProbeRTTInterval),
	}
	c.out = Output{State: Startup, PacingGain: c.pacingGain, CwndGain: c.cwndGain}
	return c
}

func (c *Controller) Enabled() bool {
	return c.cfg.Enabled
}

func (c *Controller) Snapshot() Output {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.out
}

func (c *Controller) SetEnabled(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg.Enabled = on
}

// OnSample consumes one ConnectionStats-derived sample and returns the
// pacing / cwnd that the scheduler must apply.
func (c *Controller) OnSample(s Sample) Output {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.cfg.Enabled {
		c.out = Output{
			State:      c.st,
			PacingBps:  c.cfg.MaxPacingBps,
			CwndBytes:  c.cfg.MaxPacingBps,
			PacingGain: 1,
			CwndGain:   1,
		}
		return c.out
	}
	if c.lastNs == 0 {
		c.lastNs = s.NowNs
		c.lastSent = s.BytesSent
		c.lastRecv = s.BytesReceived
		c.rtprop.Add(nonzero(s.MinRTT, s.SmoothedRTT))
		c.out.RTpropNs = c.rtprop.Min()
		return c.out
	}
	dt := s.NowNs - c.lastNs
	if dt <= 0 {
		dt = int64(time.Millisecond)
	}
	sent := delta(s.BytesSent, c.lastSent)
	recv := delta(s.BytesReceived, c.lastRecv)
	c.lastSent = s.BytesSent
	c.lastRecv = s.BytesReceived
	c.lastNs = s.NowNs

	// Delivery rate ≈ bytes ACKed / interval. Prefer received; fall back to sent.
	delivered := recv
	if delivered == 0 {
		delivered = sent
	}
	bps := delivered * uint64(time.Second) / uint64(dt)
	if bps > 0 {
		c.btlbw.Add(bps)
	}
	rt := nonzero(s.MinRTT, s.SmoothedRTT)
	if rt > 0 {
		c.rtprop.Add(rt)
	}
	btl := c.btlbw.Max()
	rtp := c.rtprop.Min()
	if rtp == 0 {
		rtp = int64(20 * time.Millisecond)
	}

	c.advance(s, btl, rtp)
	pacing := uint64(float64(maxU(btl, 1)) * c.pacingGain)
	if pacing < c.cfg.MinPacingBps {
		pacing = c.cfg.MinPacingBps
	}
	if pacing > c.cfg.MaxPacingBps {
		pacing = c.cfg.MaxPacingBps
	}
	bdp := uint64(float64(btl) * float64(rtp) / float64(time.Second))
	if bdp < minCwndPkts*defaultPacket {
		bdp = minCwndPkts * defaultPacket
	}
	cwnd := uint64(float64(bdp) * c.cwndGain)
	if c.st == ProbeRTT {
		cwnd = minCwndPkts * defaultPacket
	}
	c.out = Output{
		State:      c.st,
		PacingBps:  pacing,
		CwndBytes:  cwnd,
		BtlBwBps:   btl,
		RTpropNs:   rtp,
		PacingGain: c.pacingGain,
		CwndGain:   c.cwndGain,
		CycleIndex: c.cycle,
		FilledPipe: c.filled,
		RoundCount: c.rounds,
	}
	return c.out
}

func (c *Controller) advance(s Sample, btl uint64, rtp int64) {
	c.rounds++
	now := s.NowNs
	switch c.st {
	case Startup:
		c.pacingGain = c.cfg.StartupGain
		c.cwndGain = c.cfg.StartupGain
		if btl > 0 {
			if btl > uint64(float64(c.fullBw)*1.25) {
				c.fullBw = btl
				c.fullBwCnt = 0
			} else {
				c.fullBwCnt++
			}
			if c.fullBwCnt >= startupPlateau {
				c.filled = true
				c.enter(Drain)
			}
		}
	case Drain:
		c.pacingGain = c.cfg.DrainGain
		c.cwndGain = 2
		inflight := delta(s.BytesSent, s.BytesReceived)
		bdp := uint64(float64(btl) * float64(rtp) / float64(time.Second))
		if inflight <= bdp || c.rounds > 8 {
			c.enter(ProbeBW)
		}
	case ProbeBW:
		c.cwndGain = 2
		if now >= c.probeRTTDue {
			c.priorCwnd = c.out.CwndBytes
			c.priorState = ProbeBW
			c.probeRTTEnd = now + int64(c.cfg.ProbeRTTDuration)
			c.enter(ProbeRTT)
			return
		}
		c.pacingGain = probeBWCycle[c.cycle%len(probeBWCycle)]
		if c.rounds%2 == 0 {
			c.cycle = (c.cycle + 1) % len(probeBWCycle)
		}
	case ProbeRTT:
		c.pacingGain = 1
		c.cwndGain = 1
		if now >= c.probeRTTEnd {
			c.probeRTTDue = now + int64(c.cfg.ProbeRTTInterval)
			if c.filled {
				c.enter(ProbeBW)
			} else {
				c.enter(Startup)
			}
		}
	}
}

func (c *Controller) enter(st State) {
	c.st = st
	switch st {
	case Startup:
		c.pacingGain = c.cfg.StartupGain
		c.cwndGain = c.cfg.StartupGain
	case Drain:
		c.pacingGain = c.cfg.DrainGain
		c.cwndGain = 2
	case ProbeBW:
		c.pacingGain = 1
		c.cwndGain = 2
	case ProbeRTT:
		c.pacingGain = 1
		c.cwndGain = 1
	}
}

func delta(cur, prev uint64) uint64 {
	if cur >= prev {
		return cur - prev
	}
	return 0
}

func nonzero(a, b int64) int64 {
	if a > 0 {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxU(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
