package scheduler_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"golive/internal/alf"
	"golive/internal/config"
	"golive/internal/congestion/bbr"
	"golive/internal/scheduler"
	"golive/internal/signal"
)

func TestReliableSendErrorDoesNotReplayDeliveredItem(t *testing.T) {
	cfg := config.Config{
		QueueCap: config.QueueCap{Signal: 4, Audio: 4, Cursor: 4, Video: 4, File: 4},
		WFQ:      config.WFQWeights{Audio: 1, Cursor: 1, Video: 1, File: 1},
		BBR: config.BBRParams{
			Enabled:      false,
			StartupGain:  2,
			DrainGain:    0.5,
			BtlBwWindow:  2,
			MinPacingBps: 64 << 10,
			MaxPacingBps: 1 << 20,
		},
	}

	frame, err := signal.FrameFromJSON(
		alf.ChannelSignal,
		73,
		1_725_000_000_000,
		signal.TypePong,
		"probe-19",
		signal.Pong{ClientTs: 100, ServerTs: 120, Seq: 19},
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := alf.Encode(frame)
	if err != nil {
		t.Fatal(err)
	}

	type observed struct {
		typ string
		id  string
		seq uint64
	}
	delivered := make(chan observed, 2)
	dropped := make(chan string, 1)
	senderFailure := make(chan error, 1)
	receiver := alf.NewReassembler(time.Second, 1<<20)
	errAfterDelivery := errors.New("stream write timed out after delivery")
	attempts := 0

	s := scheduler.New(cfg, bbr.New(cfg.BBR), func(it scheduler.Item) error {
		f, decodeErr := alf.Decode(it.Payload)
		if decodeErr != nil {
			select {
			case senderFailure <- decodeErr:
			default:
			}
			return decodeErr
		}
		complete, ok, pushErr := receiver.Push(f, time.Unix(0, 0))
		if pushErr != nil {
			select {
			case senderFailure <- pushErr:
			default:
			}
			return pushErr
		}
		if !ok {
			incompleteErr := fmt.Errorf("receiver did not complete seq %d", f.Seq)
			select {
			case senderFailure <- incompleteErr:
			default:
			}
			return incompleteErr
		}
		env, decodeErr := signal.Decode(complete.Payload)
		if decodeErr != nil {
			select {
			case senderFailure <- decodeErr:
			default:
			}
			return decodeErr
		}

		attempts++
		delivered <- observed{typ: env.Type, id: env.ID, seq: complete.Seq}
		if attempts == 1 {
			return errAfterDelivery
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	if ok := s.Enqueue(scheduler.Item{
		Channel:  alf.ChannelSignal,
		Priority: alf.PrioSignal,
		Seq:      frame.Seq,
		PTS:      frame.PTS,
		Payload:  raw,
		Reliable: true,
		OnDrop: func(reason string) {
			dropped <- reason
		},
	}); !ok {
		t.Fatal("enqueue reliable signal")
	}
	go func() {
		s.Run(ctx)
		close(done)
	}()

	var first observed
	select {
	case first = <-delivered:
	case senderErr := <-senderFailure:
		cancel()
		<-done
		t.Fatalf("sender setup failed: %v", senderErr)
	case <-time.After(time.Second):
		cancel()
		<-done
		t.Fatal("first delivery timed out")
	}

	var second *observed
	var dropReason string
	select {
	case got := <-delivered:
		second = &got
	case dropReason = <-dropped:
	case senderErr := <-senderFailure:
		cancel()
		<-done
		t.Fatalf("sender failed unexpectedly: %v", senderErr)
	case <-time.After(time.Second):
		cancel()
		<-done
		t.Fatal("send error was neither dropped nor replayed")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}

	if first.typ != signal.TypePong || first.id != "probe-19" || first.seq != frame.Seq {
		t.Fatalf("first delivery = %+v", first)
	}
	if second != nil {
		t.Fatalf("same reliable response was replayed: first=%+v second=%+v", first, *second)
	}
	if dropReason != "send_error" {
		t.Fatalf("drop reason = %q, want send_error", dropReason)
	}
	stats := s.Stats()
	if stats.Sent != 0 || stats.Drops != 1 {
		t.Fatalf("stats after ambiguous send error: sent=%d drops=%d", stats.Sent, stats.Drops)
	}
}
