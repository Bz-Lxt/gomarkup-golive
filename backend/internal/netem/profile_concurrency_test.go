package netem_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"golive/internal/netem"
)

func TestNetemProfilesSwitchAtomically(t *testing.T) {
	dual := netem.NewDual(17)

	const (
		writers    = 8
		iterations = 25_000
	)
	profiles := make([]netem.Profile, writers)
	for i := range profiles {
		profiles[i] = netem.Profile{Name: fmt.Sprintf("profile-%d", i), DelayMs: i + 1}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range profiles {
		profile := profiles[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for range iterations {
				dual.Apply(profile, profile)
			}
		}()
	}

	var finished atomic.Bool
	mismatch := make(chan netem.Snapshot, 1)
	observed := make(chan struct{})
	go func() {
		defer close(observed)
		<-start
		for !finished.Load() {
			snap := dual.Snapshot()
			if snap.Uplink.Name != snap.Downlink.Name {
				select {
				case mismatch <- snap:
				default:
				}
				return
			}
		}
	}()

	close(start)
	wg.Wait()
	finished.Store(true)
	<-observed

	final := dual.Snapshot()
	if final.Uplink.Name != final.Downlink.Name {
		select {
		case mismatch <- final:
		default:
		}
	}
	select {
	case snap := <-mismatch:
		t.Fatalf("one Apply became visible as mixed profiles: uplink=%q downlink=%q", snap.Uplink.Name, snap.Downlink.Name)
	default:
	}
}
