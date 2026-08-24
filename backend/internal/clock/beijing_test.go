package clock

import (
	"strings"
	"testing"
	"time"
)

func TestNowIsBeijing(t *testing.T) {
	n := Now()
	name, offset := n.Zone()
	if offset != 8*3600 {
		t.Fatalf("offset=%d name=%s want +8h", offset, name)
	}
}

func TestFormatLayout(t *testing.T) {
	ts := time.Date(2026, 8, 23, 16, 7, 0, 0, beijingTZ)
	got := Format(ts)
	if got != "2026-08-23 16:07:00" {
		t.Fatalf("Format = %q", got)
	}
	if Format(time.Time{}) != "" {
		t.Fatal("zero time should format empty")
	}
}

func TestToBeijingFromUTC(t *testing.T) {
	utc := time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC)
	bj := ToBeijing(utc)
	if bj.Hour() != 16 {
		t.Fatalf("hour=%d want 16", bj.Hour())
	}
}

func TestMonoIncreases(t *testing.T) {
	m := NewMono()
	a := m.Tick()
	time.Sleep(2 * time.Millisecond)
	b := m.Tick()
	if b <= a {
		t.Fatalf("mono not increasing: %d then %d", a, b)
	}
}

func TestLayoutMatchesRequirement(t *testing.T) {
	if !strings.Contains(Layout, "15:04:05") {
		t.Fatal("layout must include time of day")
	}
}
