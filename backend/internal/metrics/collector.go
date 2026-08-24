package metrics

import (
	"sync"
	"time"

	"github.com/quic-go/quic-go"

	"golive/internal/clock"
)

// Source abstracts quic.Conn so tests don't need a real handshake.
type Source interface {
	ConnectionStats() quic.ConnectionStats
	ConnectionState() quic.ConnectionState
}

type Collector struct {
	mu       sync.Mutex
	last     quic.ConnectionStats
	lastAt   time.Time
	have     bool
	sendBps  uint64
	recvBps  uint64
	lossRate float64
	appRTT   float64
	videoN   ring
	audioN   ring
	cursorN  ring
	dropV    uint64
	dropA    uint64
	dropC    uint64
}

func NewCollector() *Collector {
	return &Collector{
		videoN:  ring{window: time.Second},
		audioN:  ring{window: time.Second},
		cursorN: ring{window: time.Second},
	}
}

func (c *Collector) NoteAppRTT(ms float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.appRTT = ms
}

func (c *Collector) NoteFrame(kind string) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	switch kind {
	case "video":
		c.videoN.add(now)
	case "audio":
		c.audioN.add(now)
	case "cursor":
		c.cursorN.add(now)
	}
}

func (c *Collector) NoteDrop(kind string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch kind {
	case "video":
		c.dropV++
	case "audio":
		c.dropA++
	case "cursor":
		c.dropC++
	}
}

func (c *Collector) Observe(src Source) (quic.ConnectionStats, quic.ConnectionState, uint64, uint64, float64) {
	st := src.ConnectionStats()
	cs := src.ConnectionState()
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.have {
		dt := now.Sub(c.lastAt).Seconds()
		if dt > 0 {
			ds := delta(st.BytesSent, c.last.BytesSent)
			dr := delta(st.BytesReceived, c.last.BytesReceived)
			c.sendBps = uint64(float64(ds) / dt)
			c.recvBps = uint64(float64(dr) / dt)
			dSent := delta(st.PacketsSent, c.last.PacketsSent)
			// PacketsLost is non-monotonic; use instantaneous |delta| / sent.
			var dLost uint64
			if st.PacketsLost >= c.last.PacketsLost {
				dLost = st.PacketsLost - c.last.PacketsLost
			}
			if dSent > 0 {
				c.lossRate = float64(dLost) / float64(dSent)
			} else {
				c.lossRate = 0
			}
		}
	}
	c.last = st
	c.lastAt = now
	c.have = true
	return st, cs, c.sendBps, c.recvBps, c.lossRate
}

func (c *Collector) Fill(snap *Snapshot) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	snap.AppRTTMs = c.appRTT
	snap.SendBps = c.sendBps
	snap.RecvBps = c.recvBps
	snap.LossRate = c.lossRate
	snap.VideoFPS = c.videoN.rate(now)
	snap.AudioFPS = c.audioN.rate(now)
	snap.CursorFPS = c.cursorN.rate(now)
	snap.DroppedVideo = c.dropV
	snap.DroppedAudio = c.dropA
	snap.DroppedCursor = c.dropC
	snap.Ts = clock.UnixMilliBeijing()
	snap.TsText = clock.Format(clock.Now())
}

func delta(cur, prev uint64) uint64 {
	if cur >= prev {
		return cur - prev
	}
	return 0
}

func DurationMs(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}
