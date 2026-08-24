package bbr

// maxFilter keeps the maximum of values inserted in the last `window` samples.
type maxFilter struct {
	window int
	vals   []uint64
	pos    int
	filled bool
}

func newMaxFilter(window int) *maxFilter {
	if window < 2 {
		window = 2
	}
	return &maxFilter{window: window, vals: make([]uint64, window)}
}

func (f *maxFilter) Add(v uint64) uint64 {
	f.vals[f.pos] = v
	f.pos = (f.pos + 1) % f.window
	if f.pos == 0 {
		f.filled = true
	}
	return f.Max()
}

func (f *maxFilter) Max() uint64 {
	n := f.window
	if !f.filled {
		n = f.pos
	}
	var m uint64
	for i := 0; i < n; i++ {
		if f.vals[i] > m {
			m = f.vals[i]
		}
	}
	return m
}

// minFilter keeps the minimum of values inserted in the last `window` samples.
// Zero is treated as missing (quic RTT starts at 0 before first sample).
type minFilter struct {
	window int
	vals   []int64
	pos    int
	filled bool
}

func newMinFilter(window int) *minFilter {
	if window < 2 {
		window = 2
	}
	return &minFilter{window: window, vals: make([]int64, window)}
}

func (f *minFilter) Add(v int64) int64 {
	f.vals[f.pos] = v
	f.pos = (f.pos + 1) % f.window
	if f.pos == 0 {
		f.filled = true
	}
	return f.Min()
}

func (f *minFilter) Min() int64 {
	n := f.window
	if !f.filled {
		n = f.pos
	}
	var m int64
	have := false
	for i := 0; i < n; i++ {
		if f.vals[i] <= 0 {
			continue
		}
		if !have || f.vals[i] < m {
			m = f.vals[i]
			have = true
		}
	}
	return m
}
