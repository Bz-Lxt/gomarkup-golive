package alf

import "fmt"

// ChannelID identifies a logical media/control channel.
type ChannelID uint8

const (
	ChannelSignal ChannelID = 0
	ChannelAudio  ChannelID = 1
	ChannelCursor ChannelID = 2
	ChannelVideo  ChannelID = 3
	ChannelFile   ChannelID = 4
)

func (c ChannelID) String() string {
	switch c {
	case ChannelSignal:
		return "signal"
	case ChannelAudio:
		return "audio"
	case ChannelCursor:
		return "cursor"
	case ChannelVideo:
		return "video"
	case ChannelFile:
		return "file"
	default:
		return fmt.Sprintf("channel(%d)", uint8(c))
	}
}

func ParseChannel(name string) (ChannelID, error) {
	switch name {
	case "signal":
		return ChannelSignal, nil
	case "audio":
		return ChannelAudio, nil
	case "cursor":
		return ChannelCursor, nil
	case "video":
		return ChannelVideo, nil
	case "file":
		return ChannelFile, nil
	default:
		return 0, fmt.Errorf("unknown channel %q", name)
	}
}

func (c ChannelID) Valid() bool {
	return c <= ChannelFile
}

// Kind is the WebTransport primitive used for a channel.
type Kind uint8

const (
	KindBidi     Kind = 1
	KindUni      Kind = 2
	KindDatagram Kind = 3
)

func (k Kind) String() string {
	switch k {
	case KindBidi:
		return "bidi"
	case KindUni:
		return "uni"
	case KindDatagram:
		return "datagram"
	default:
		return fmt.Sprintf("kind(%d)", uint8(k))
	}
}

func (k Kind) Valid() bool {
	return k == KindBidi || k == KindUni || k == KindDatagram
}

// Priority is lower number = higher priority. Matches Requirements §4.4.
type Priority uint8

const (
	PrioSignal Priority = 0
	PrioAudio  Priority = 1
	PrioCursor Priority = 1
	PrioVideo  Priority = 2
	PrioFile   Priority = 3
)

func DefaultPriority(ch ChannelID) Priority {
	switch ch {
	case ChannelSignal:
		return PrioSignal
	case ChannelAudio, ChannelCursor:
		return PrioAudio
	case ChannelVideo:
		return PrioVideo
	default:
		return PrioFile
	}
}

func DefaultKind(ch ChannelID) Kind {
	switch ch {
	case ChannelSignal, ChannelFile:
		return KindBidi
	case ChannelVideo:
		return KindUni
	default:
		return KindDatagram
	}
}

func Reliable(ch ChannelID) bool {
	return ch == ChannelSignal || ch == ChannelFile || ch == ChannelVideo
}
