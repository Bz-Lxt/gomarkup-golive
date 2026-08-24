package signal

import (
	"encoding/json"
	"fmt"

	"golive/internal/alf"
	"golive/internal/metrics"
	"golive/internal/netem"
)

const (
	TypeHello       = "hello"
	TypeWelcome     = "welcome"
	TypeMetrics     = "metrics"
	TypeNetemSet    = "netem.set"
	TypeNetemOK     = "netem.ok"
	TypeBBRSet      = "bbr.set"
	TypeBBROk       = "bbr.ok"
	TypePing        = "ping"
	TypePong        = "pong"
	TypeFileBegin   = "file.begin"
	TypeFileChunk   = "file.chunk"
	TypeFileDone    = "file.done"
	TypeFileAck     = "file.ack"
	TypeError       = "error"
	TypeRoomJoin    = "room.join"
	TypeRoomEvent   = "room.event"
)

type Envelope struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	Session   string          `json:"session,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func Encode(typ, id string, payload any) ([]byte, error) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("signal marshal payload: %w", err)
		}
		raw = b
	}
	return json.Marshal(Envelope{Type: typ, ID: id, Payload: raw})
}

func Decode(b []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(b, &e); err != nil {
		return e, fmt.Errorf("signal envelope: %w", err)
	}
	if e.Type == "" {
		return e, fmt.Errorf("signal: missing type")
	}
	return e, nil
}

type Hello struct {
	Room     string `json:"room"`
	Client   string `json:"client"`
	Mode     string `json:"mode"` // echo | room
	UserAgent string `json:"user_agent"`
}

type Welcome struct {
	SessionID   string   `json:"session_id"`
	Room        string   `json:"room"`
	Mode        string   `json:"mode"`
	WTURL       string   `json:"wt_url"`
	MaxDatagram int      `json:"max_datagram"`
	Channels    []string `json:"channels"`
	BBRLayer    string   `json:"bbr_layer"`
	ServerTime  string   `json:"server_time"`
}

type NetemSet struct {
	Preset   string `json:"preset"`
	Uplink   *netem.Profile `json:"uplink,omitempty"`
	Downlink *netem.Profile `json:"downlink,omitempty"`
}

type BBRSet struct {
	Enabled bool `json:"enabled"`
}

type Ping struct {
	ClientTs int64 `json:"client_ts"`
	Seq      uint64 `json:"seq"`
}

type Pong struct {
	ClientTs int64 `json:"client_ts"`
	ServerTs int64 `json:"server_ts"`
	Seq      uint64 `json:"seq"`
}

type FileBegin struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Chunks int    `json:"chunks"`
}

type FileAck struct {
	Name     string `json:"name"`
	Received int64  `json:"received"`
	SHA256   string `json:"sha256"`
	Match    bool   `json:"match"`
	Error    string `json:"error,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func FrameFromJSON(ch alf.ChannelID, seq uint64, pts int64, typ, id string, payload any) (alf.Frame, error) {
	body, err := Encode(typ, id, payload)
	if err != nil {
		return alf.Frame{}, err
	}
	return alf.Frame{
		Channel:   ch,
		Kind:      alf.DefaultKind(ch),
		Priority:  alf.DefaultPriority(ch),
		Seq:       seq,
		PTS:       pts,
		Flags:     alf.FlagEndOfMessage,
		FragIdx:   0,
		FragTotal: 1,
		Payload:   body,
	}, nil
}

func MetricsPayload(s metrics.Snapshot) any { return s }
