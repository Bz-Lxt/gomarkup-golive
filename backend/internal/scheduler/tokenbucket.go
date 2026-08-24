package scheduler

import (
	"sync"
	"time"
)

// Bucket is a byte-oriented token bucket driven by BBR pacing_rate.
type Bucket struct {
	mu       sync.Mutex
	rate     uint64
	burst    uint64
	tokens   float64
	last     time.Time
	disabled bool
}

func NewBucket(rate, burst uint64) *Bucket {
	if rate == 0 {
		rate = 64 * 1024
	}
	if burst == 0 {
		burst = rate / 4
		if burst < 8*1024 {
			burst = 8 * 1024
		}
	}
	return &Bucket{rate: rate, burst: burst, tokens: float64(burst), last: time.Now()}
}

func (b *Bucket) SetRate(rate uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if rate == 0 {
		return
	}
	b.rate = rate
	if b.burst < rate/8 {
		b.burst = rate / 8
	}
}

func (b *Bucket) SetDisabled(off bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.disabled = off
}

func (b *Bucket) Rate() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.rate
}

// Allow reports whether n bytes may leave now, consuming tokens if so.
func (b *Bucket) Allow(n int) bool {
	if n <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.disabled {
		return true
	}
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * float64(b.rate)
	if b.tokens > float64(b.burst) {
		b.tokens = float64(b.burst)
	}
	need := float64(n)
	if b.tokens >= need {
		b.tokens -= need
		return true
	}
	return false
}

// WaitNs is how long until n bytes would be allowed (best-effort).
func (b *Bucket) WaitNs(n int) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.disabled || b.tokens >= float64(n) {
		return 0
	}
	deficit := float64(n) - b.tokens
	sec := deficit / float64(b.rate)
	return int64(sec * 1e9)
}
