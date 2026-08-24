package netem

import (
	"math/rand"
	"sync"
)

// RNG is a mutex-protected seeded source. Same seed + same decision
// sequence is reproducible (Requirements A-07).
type RNG struct {
	mu  sync.Mutex
	src *rand.Rand
}

func NewRNG(seed int64) *RNG {
	return &RNG{src: rand.New(rand.NewSource(seed))}
}

func (r *RNG) Float64() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.src.Float64()
}

func (r *RNG) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.src.Intn(n)
}

func (r *RNG) Drop(pct float64) bool {
	if pct <= 0 {
		return false
	}
	if pct >= 100 {
		return true
	}
	return r.Float64()*100 < pct
}

func (r *RNG) Jitter(baseMs, jitterMs int) int {
	if jitterMs <= 0 {
		return baseMs
	}
	delta := r.Intn(jitterMs*2+1) - jitterMs
	out := baseMs + delta
	if out < 0 {
		return 0
	}
	return out
}
