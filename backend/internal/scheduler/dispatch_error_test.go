package scheduler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"golive/internal/alf"
	"golive/internal/config"
	"golive/internal/congestion/bbr"
	"golive/internal/scheduler"
)

func TestSchedulerReportsFailedItemBeforeNextDispatch(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.BBR.Enabled = false
	cfg.QueueCap.Signal = 2

	secondStarted := make(chan struct{})
	releaseSecond := make(chan struct{})
	dropped := make(chan string, 1)
	s := scheduler.New(*cfg, bbr.New(cfg.BBR), func(it scheduler.Item) error {
		switch string(it.Payload) {
		case "first":
			return errors.New("stream reset")
		case "second":
			close(secondStarted)
			<-releaseSecond
		}
		return nil
	})

	if !s.Enqueue(scheduler.Item{
		Channel:  alf.ChannelSignal,
		Payload:  []byte("first"),
		Reliable: true,
		OnDrop:   func(reason string) { dropped <- reason },
	}) || !s.Enqueue(scheduler.Item{Channel: alf.ChannelSignal, Payload: []byte("second"), Reliable: true}) {
		t.Fatal("failed to enqueue dispatch batch")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		close(releaseSecond)
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("scheduler did not stop")
		}
	})

	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("second dispatch did not start")
	}
	select {
	case reason := <-dropped:
		if reason != "send_error" {
			t.Fatalf("drop reason = %q, want send_error", reason)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("first dispatch failure was not reported before the second dispatch blocked")
	}
	if got := s.Stats().Drops; got != 1 {
		t.Fatalf("drops = %d, want 1", got)
	}
}
