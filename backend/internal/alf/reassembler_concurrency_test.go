package alf_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"golive/internal/alf"
)

func TestConcurrentDuplicateFragmentCompletesOnce(t *testing.T) {
	const (
		payloadSize  = 8 << 20
		fragmentSize = 60 << 10
		workers      = 8
	)
	payload := bytes.Repeat([]byte("video-frame-"), payloadSize/len("video-frame-")+1)[:payloadSize]
	fragments, err := alf.Split(alf.Frame{
		Channel:  alf.ChannelVideo,
		Kind:     alf.KindUni,
		Priority: alf.PrioVideo,
		Seq:      901,
	}, payload, fragmentSize)
	if err != nil {
		t.Fatal(err)
	}

	r := alf.NewReassembler(time.Second, payloadSize)
	now := time.Now()
	for _, fragment := range fragments[:len(fragments)-1] {
		if _, complete, err := r.Push(fragment, now); err != nil || complete {
			t.Fatalf("preload complete=%v err=%v", complete, err)
		}
	}

	type result struct {
		frame    alf.Frame
		complete bool
		err      error
	}
	results := make(chan result, workers)
	start := make(chan struct{})
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(workers)
	done.Add(workers)
	tail := fragments[len(fragments)-1]
	for i := 0; i < workers; i++ {
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			frame, complete, err := r.Push(tail, now)
			results <- result{frame: frame, complete: complete, err: err}
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	close(results)

	completed := 0
	for got := range results {
		if got.err != nil {
			t.Fatal(got.err)
		}
		if !got.complete {
			continue
		}
		completed++
		if !bytes.Equal(got.frame.Payload, payload) {
			t.Fatalf("completed payload mismatch: got %d bytes", len(got.frame.Payload))
		}
	}
	if completed != 1 {
		t.Fatalf("one logical message completed %d times", completed)
	}
}
