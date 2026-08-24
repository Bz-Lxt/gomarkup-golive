package scheduler_test

import (
	"context"
	"testing"
	"time"

	"golive/internal/alf"
	"golive/internal/config"
	"golive/internal/congestion/bbr"
	"golive/internal/scheduler"
)

func TestActiveSchedulerOutlivesSessionIdleThreshold(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.BBR.Enabled = false
	cfg.SessionIdle = 80 * time.Millisecond
	cfg.QueueCap.Audio = 64

	sent := make(chan uint64, 1)
	s := scheduler.New(*cfg, bbr.New(cfg.BBR), func(it scheduler.Item) error {
		sent <- it.Seq
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(runDone)
	}()
	defer func() {
		cancel()
		select {
		case <-runDone:
		case <-time.After(time.Second):
			t.Error("scheduler did not stop after cancellation")
		}
	}()

	end := time.Now().Add(3 * cfg.SessionIdle)
	for seq := uint64(1); time.Now().Before(end); seq++ {
		if ok := s.Enqueue(scheduler.Item{
			Channel: alf.ChannelAudio,
			Seq:     seq,
			Payload: []byte{byte(seq)},
		}); !ok {
			t.Fatalf("active item %d was rejected", seq)
		}
		select {
		case got := <-sent:
			if got != seq {
				t.Fatalf("sent sequence %d, want %d", got, seq)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("active item %d was accepted but not sent", seq)
		}
		time.Sleep(cfg.SessionIdle / 5)
	}
}
