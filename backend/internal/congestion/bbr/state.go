package bbr

import "fmt"

// State is the BBR v1 four-state machine.
// This runs at the APPLICATION scheduler, not inside quic-go's
// internal Cubic sender. See Requirements C-01.
type State int

const (
	Startup State = iota
	Drain
	ProbeBW
	ProbeRTT
)

func (s State) String() string {
	switch s {
	case Startup:
		return "Startup"
	case Drain:
		return "Drain"
	case ProbeBW:
		return "ProbeBW"
	case ProbeRTT:
		return "ProbeRTT"
	default:
		return fmt.Sprintf("State(%d)", int(s))
	}
}

// Sample is a telemetry snapshot consumed from quic.ConnectionStats
// (or a test double). Units: bytes, packets, nanoseconds for RTT.
type Sample struct {
	MinRTT        int64
	LatestRTT     int64
	SmoothedRTT   int64
	MeanDeviation int64
	BytesSent     uint64
	BytesReceived uint64
	PacketsSent   uint64
	PacketsLost   uint64
	NowNs         int64
}

// Output is applied to the token-bucket pacer.
type Output struct {
	State        State   `json:"state"`
	PacingBps    uint64  `json:"pacing_bps"`
	CwndBytes    uint64  `json:"cwnd_bytes"`
	BtlBwBps     uint64  `json:"btlbw_bps"`
	RTpropNs     int64   `json:"rtprop_ns"`
	PacingGain   float64 `json:"pacing_gain"`
	CwndGain     float64 `json:"cwnd_gain"`
	CycleIndex   int     `json:"cycle_index"`
	FilledPipe   bool    `json:"filled_pipe"`
	RoundCount   uint64  `json:"round_count"`
}
