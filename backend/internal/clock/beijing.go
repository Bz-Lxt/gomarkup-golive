// Package clock provides GMT+8 Beijing time helpers.
// All user-visible timestamps and ALF pts use these functions.
// Never call time.Now().UTC() for persisted or displayed values.
package clock

import (
	"sync/atomic"
	"time"
)

const Layout = "2006-01-02 15:04:05"

var beijingTZ = time.FixedZone("CST", 8*3600)

// Now returns the current wall clock in Beijing, with location attached.
func Now() time.Time {
	return time.Now().In(beijingTZ)
}

// NowNaive returns Beijing wall clock with tzinfo stripped (SQL-friendly).
func NowNaive() time.Time {
	return Now().Truncate(time.Second)
}

// Format formats t in Beijing using Layout. t may be any location.
func Format(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(beijingTZ).Format(Layout)
}

// UnixMilliBeijing returns the current Unix millisecond timestamp.
// Used as ALF pts. This is timezone-independent (epoch based) but
// derived from the same clock source as Now() for consistency.
func UnixMilliBeijing() int64 {
	return Now().UnixMilli()
}

// ToBeijing converts any time to Beijing location.
func ToBeijing(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.In(beijingTZ)
}

// Mono is a process-local monotonic millisecond counter used for
// sequencing when wall clock jumps. Zero value is usable after first Tick.
type Mono struct {
	base  atomic.Int64
	start time.Time
}

func NewMono() *Mono {
	m := &Mono{start: time.Now()}
	m.base.Store(UnixMilliBeijing())
	return m
}

// Tick returns a strictly increasing millisecond timestamp.
func (m *Mono) Tick() int64 {
	if m == nil {
		return UnixMilliBeijing()
	}
	elapsed := time.Since(m.start).Milliseconds()
	return m.base.Load() + elapsed
}
