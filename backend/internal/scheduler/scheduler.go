// Package scheduler implements a per-session multiplexor:
// strict priority for signal (P0) plus weighted fair queueing among
// audio/cursor/video/file, paced by a BBR-driven token bucket.
package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"golive/internal/alf"
	"golive/internal/config"
	"golive/internal/congestion/bbr"
	"golive/internal/logging"
)

type Sender func(it Item) error

type Scheduler struct {
	cfg    config.Config
	bank   *bank
	bucket *Bucket
	bbr    *bbr.Controller
	send   Sender

	wfqCredit map[alf.ChannelID]int
	weights   map[alf.ChannelID]int

	wake   chan struct{}
	closed atomic.Bool
	drops  atomic.Uint64
	sent   atomic.Uint64

	mu sync.Mutex
}

func New(cfg config.Config, ctrl *bbr.Controller, send Sender) *Scheduler {
	s := &Scheduler{
		cfg:    cfg,
		bank:   newBank(cfg.QueueCap),
		bucket: NewBucket(cfg.BBR.MinPacingBps, cfg.BBR.MinPacingBps),
		bbr:    ctrl,
		send:   send,
		wake:   make(chan struct{}, 1),
		wfqCredit: map[alf.ChannelID]int{
			alf.ChannelAudio:  0,
			alf.ChannelCursor: 0,
			alf.ChannelVideo:  0,
			alf.ChannelFile:   0,
		},
		weights: map[alf.ChannelID]int{
			alf.ChannelAudio:  cfg.WFQ.Audio,
			alf.ChannelCursor: cfg.WFQ.Cursor,
			alf.ChannelVideo:  cfg.WFQ.Video,
			alf.ChannelFile:   cfg.WFQ.File,
		},
	}
	s.bucket.SetDisabled(!cfg.BBR.Enabled)
	return s
}

func (s *Scheduler) Enqueue(it Item) bool {
	if s.closed.Load() {
		if it.OnDrop != nil {
			it.OnDrop("scheduler_closed")
		}
		return false
	}
	s.bank.mu.Lock()
	ok := s.bank.of(it.Channel).enqueue(it)
	s.bank.mu.Unlock()
	if !ok {
		s.drops.Add(1)
		return false
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return true
}

func (s *Scheduler) Run(ctx context.Context) {
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()
	for {
		if s.closed.Load() {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
			s.drainOnce()
		case <-tick.C:
			s.drainOnce()
		}
	}
}

func (s *Scheduler) Close() {
	s.closed.Store(true)
}

func (s *Scheduler) ApplyBBR(out bbr.Output) {
	s.bucket.SetDisabled(!s.bbr.Enabled())
	if out.PacingBps > 0 {
		s.bucket.SetRate(out.PacingBps)
	}
}

func (s *Scheduler) Stats() Stats {
	return Stats{
		Sent:    s.sent.Load(),
		Drops:   s.drops.Load(),
		Pacing:  s.bucket.Rate(),
		Queues:  s.bank.Snapshot(),
		BBR:     s.bbr.Snapshot(),
	}
}

type Stats struct {
	Sent   uint64         `json:"sent"`
	Drops  uint64         `json:"drops"`
	Pacing uint64         `json:"pacing_bps"`
	Queues []QueueSnap    `json:"queues"`
	BBR    bbr.Output     `json:"bbr"`
}

func (s *Scheduler) drainOnce() {
	for {
		it, ok := s.pick()
		if !ok {
			return
		}
		if !s.bucket.Allow(it.Size()) {
			// put back at front
			s.bank.mu.Lock()
			q := s.bank.of(it.Channel)
			q.items = append([]Item{it}, q.items...)
			s.bank.mu.Unlock()
			return
		}
		if err := s.send(it); err != nil {
			logging.L().Debug("scheduler send failed", "ch", it.Channel.String(), "err", err)
			if it.Reliable {
				s.Enqueue(it)
				return
			}
			if it.OnDrop != nil {
				it.OnDrop("send_error")
			}
			s.drops.Add(1)
			continue
		}
		s.sent.Add(1)
	}
}

func (s *Scheduler) pick() (Item, bool) {
	s.bank.mu.Lock()
	defer s.bank.mu.Unlock()

	// P0 signal is always first (never starved by WFQ).
	if it, ok := s.bank.of(alf.ChannelSignal).dequeue(); ok {
		return it, true
	}

	// Replenish credits.
	ready := []alf.ChannelID{alf.ChannelAudio, alf.ChannelCursor, alf.ChannelVideo, alf.ChannelFile}
	any := false
	for _, ch := range ready {
		if s.bank.of(ch).depth() > 0 {
			any = true
			s.wfqCredit[ch] += s.weights[ch]
		}
	}
	if !any {
		return Item{}, false
	}
	var best alf.ChannelID
	bestCredit := -1
	for _, ch := range ready {
		if s.bank.of(ch).depth() == 0 {
			continue
		}
		if s.wfqCredit[ch] > bestCredit {
			bestCredit = s.wfqCredit[ch]
			best = ch
		}
	}
	it, ok := s.bank.of(best).dequeue()
	if ok {
		cost := it.Size()
		if cost < 1 {
			cost = 1
		}
		s.wfqCredit[best] -= cost
	}
	return it, ok
}
