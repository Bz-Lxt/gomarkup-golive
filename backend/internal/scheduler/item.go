package scheduler

import (
	"golive/internal/alf"
)

// Item is a unit waiting to leave the host. Payload is already encoded ALF.
type Item struct {
	Channel  alf.ChannelID
	Priority alf.Priority
	Seq      uint64
	PTS      int64
	Payload  []byte
	Reliable bool
	OnDrop   func(reason string)
}

func (it Item) Size() int { return len(it.Payload) }
