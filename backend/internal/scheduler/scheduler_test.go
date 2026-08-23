package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"golive/internal/alf"
	"golive/internal/config"
	"golive/internal/congestion/bbr"
)

func testCfg() config.Config {
	c, err := config.Load()
	if err != nil {
		panic(err)
	}
	c.QueueCap = config.QueueCap{Signal: 8, Audio: 8, Cursor: 8, Video: 8, File: 8}
	c.WFQ = config.WFQWeights{Audio: 4, Cursor: 4, Video: 2, File: 1}
	c.BBR.Enabled = false
	return *c
}

func TestSignalAlwaysFirst(t *testing.T) {
	var order []alf.ChannelID
	cfg := testCfg()
	ctrl := bbr.New(cfg.BBR)
	s := New(cfg, ctrl, func(it Item) error {
		order = append(order, it.Channel)
		return nil
	})
	s.Enqueue(Item{Channel: alf.ChannelFile, Payload: []byte("f")})
	s.Enqueue(Item{Channel: alf.ChannelVideo, Payload: []byte("v")})
	s.Enqueue(Item{Channel: alf.ChannelSignal, Payload: []byte("s")})
	s.drainOnce()
	if len(order) == 0 || order[0] != alf.ChannelSignal {
		t.Fatalf("order=%v", order)
	}
}

func TestQueueDropCounts(t *testing.T) {
	cfg := testCfg()
	cfg.QueueCap.Cursor = 1
	ctrl := bbr.New(cfg.BBR)
	var drops atomic.Int32
	s := New(cfg, ctrl, func(Item) error { return nil })
	ok1 := s.Enqueue(Item{Channel: alf.ChannelCursor, Payload: []byte("a")})
	ok2 := s.Enqueue(Item{Channel: alf.ChannelCursor, Payload: []byte("b"), OnDrop: func(string) { drops.Add(1) }})
	if !ok1 || ok2 {
		t.Fatalf("ok1=%v ok2=%v", ok1, ok2)
	}
	if drops.Load() != 1 {
		t.Fatalf("drops=%d", drops.Load())
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	cfg := testCfg()
	s := New(cfg, bbr.New(cfg.BBR), func(Item) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("leak")
	}
}

func TestWFQGivesFileAShare(t *testing.T) {
	cfg := testCfg()
	cfg.BBR.Enabled = false
	var file, video int
	s := New(cfg, bbr.New(cfg.BBR), func(it Item) error {
		switch it.Channel {
		case alf.ChannelFile:
			file++
		case alf.ChannelVideo:
			video++
		}
		return nil
	})
	for i := 0; i < 20; i++ {
		s.Enqueue(Item{Channel: alf.ChannelVideo, Payload: bytesN(100)})
		s.Enqueue(Item{Channel: alf.ChannelFile, Payload: bytesN(100)})
	}
	s.drainOnce()
	if file == 0 {
		t.Fatal("file starved")
	}
	if video == 0 {
		t.Fatal("video starved")
	}
}

func bytesN(n int) []byte { return make([]byte, n) }
