package metrics

import "time"

type ring struct {
	window time.Duration
	times  []time.Time
}

func (r *ring) add(t time.Time) {
	r.times = append(r.times, t)
	r.gc(t)
}

func (r *ring) gc(now time.Time) {
	cut := now.Add(-r.window)
	i := 0
	for i < len(r.times) && r.times[i].Before(cut) {
		i++
	}
	if i > 0 {
		r.times = r.times[i:]
	}
}

func (r *ring) rate(now time.Time) float64 {
	r.gc(now)
	return float64(len(r.times))
}
