package metrics

import (
	"golive/internal/congestion/bbr"
	"golive/internal/scheduler"
)

// Snapshot is the 10 Hz payload pushed on the signal channel.
// Source A is ConnectionStats (transport truth). Source B is filled by
// the session's ping/pong path (app RTT).
type Snapshot struct {
	Ts               int64   `json:"ts"`
	TsText           string  `json:"ts_text"`
	SessionID        string  `json:"session_id"`
	MinRTTMs         float64 `json:"min_rtt_ms"`
	LatestRTTMs      float64 `json:"latest_rtt_ms"`
	SmoothedRTTMs    float64 `json:"smoothed_rtt_ms"`
	MeanDeviationMs  float64 `json:"mean_deviation_ms"`
	AppRTTMs         float64 `json:"app_rtt_ms"`
	BytesSent        uint64  `json:"bytes_sent"`
	BytesReceived    uint64  `json:"bytes_received"`
	PacketsSent      uint64  `json:"packets_sent"`
	PacketsReceived  uint64  `json:"packets_received"`
	PacketsLost      uint64  `json:"packets_lost"`
	BytesLost        uint64  `json:"bytes_lost"`
	SendBps          uint64  `json:"send_bps"`
	RecvBps          uint64  `json:"recv_bps"`
	LossRate         float64 `json:"loss_rate"`
	VideoFPS         float64 `json:"video_fps"`
	AudioFPS         float64 `json:"audio_fps"`
	CursorFPS        float64 `json:"cursor_fps"`
	DroppedVideo     uint64  `json:"dropped_video"`
	DroppedAudio     uint64  `json:"dropped_audio"`
	DroppedCursor    uint64  `json:"dropped_cursor"`
	QUICVersion      uint32  `json:"quic_version"`
	GSO              bool    `json:"gso"`
	DatagramRemote   bool    `json:"datagram_remote"`
	DatagramLocal    bool    `json:"datagram_local"`
	MaxDatagram      int     `json:"max_datagram"`
	BBR              bbr.Output `json:"bbr"`
	Scheduler        scheduler.Stats `json:"scheduler"`
	NetemLossUp      float64 `json:"netem_loss_up"`
	NetemLossDown    float64 `json:"netem_loss_down"`
}
