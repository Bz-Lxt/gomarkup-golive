package netem

import (
	"fmt"
	"sync"
	"time"
)

// Profile is a runtime-switchable impairment recipe applied independently
// on uplink (server→client writes) and downlink (client→server reads).
type Profile struct {
	Name         string  `json:"name"`
	LossPct      float64 `json:"loss_pct"`
	DelayMs      int     `json:"delay_ms"`
	JitterMs     int     `json:"jitter_ms"`
	ReorderPct   float64 `json:"reorder_pct"`
	ReorderDelay int     `json:"reorder_delay_ms"`
}

var presets = map[string]Profile{
	"0": {
		Name: "0", LossPct: 0,
	},
	"10": {
		Name: "10", LossPct: 10, DelayMs: 8, JitterMs: 4,
	},
	"30": {
		Name: "30", LossPct: 30, DelayMs: 20, JitterMs: 12, ReorderPct: 5, ReorderDelay: 8,
	},
	"50": {
		Name: "50", LossPct: 50, DelayMs: 40, JitterMs: 20, ReorderPct: 10, ReorderDelay: 15,
	},
}

func Preset(name string) (Profile, error) {
	p, ok := presets[name]
	if !ok {
		return Profile{}, fmt.Errorf("unknown netem preset %q (want 0|10|30|50)", name)
	}
	return p, nil
}

func PresetNames() []string {
	return []string{"0", "10", "30", "50"}
}

// Dual holds independent up/down profiles plus a seed for reproducibility.
type Dual struct {
	mu     sync.RWMutex
	up     Profile
	down   Profile
	seed   int64
	switched time.Time
}

func NewDual(seed int64) *Dual {
	p := presets["0"]
	return &Dual{up: p, down: p, seed: seed, switched: time.Now()}
}

func (d *Dual) Seed() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.seed
}

func (d *Dual) Snapshot() Snapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return Snapshot{
		Uplink:    d.up,
		Downlink:  d.down,
		Seed:      d.seed,
		SwitchedAt: d.switched.UnixMilli(),
	}
}

type Snapshot struct {
	Uplink     Profile `json:"uplink"`
	Downlink   Profile `json:"downlink"`
	Seed       int64   `json:"seed"`
	SwitchedAt int64   `json:"switched_at_ms"`
}

// Apply replaces both uplink and downlink profiles as a single version.
// The write lock is held across both fields so a concurrent Snapshot or
// Apply can never observe a torn state (uplink from one PUT, downlink
// from another). Each PUT is therefore one atomic version visible to
// readers — including GET /api/v1/netem.
func (d *Dual) Apply(up, down Profile) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.up = up
	d.down = down
	d.switched = time.Now()
}

func (d *Dual) ApplyPreset(name string) error {
	p, err := Preset(name)
	if err != nil {
		return err
	}
	d.Apply(p, p)
	return nil
}

func (d *Dual) Uplink() Profile {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.up
}

func (d *Dual) Downlink() Profile {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.down
}

func (p Profile) Validate() error {
	if p.LossPct < 0 || p.LossPct > 100 {
		return fmt.Errorf("loss_pct %v out of 0..100", p.LossPct)
	}
	if p.DelayMs < 0 || p.JitterMs < 0 || p.ReorderDelay < 0 {
		return fmt.Errorf("negative delay")
	}
	if p.ReorderPct < 0 || p.ReorderPct > 100 {
		return fmt.Errorf("reorder_pct %v out of 0..100", p.ReorderPct)
	}
	return nil
}
