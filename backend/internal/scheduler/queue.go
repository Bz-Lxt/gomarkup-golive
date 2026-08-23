package scheduler

import (
	"sync"

	"golive/internal/alf"
	"golive/internal/config"
)

type queue struct {
	ch   alf.ChannelID
	cap  int
	items []Item
	drops uint64
	enq   uint64
	deq   uint64
	bytes uint64
}

func newQueue(ch alf.ChannelID, cap int) *queue {
	if cap < 1 {
		cap = 8
	}
	return &queue{ch: ch, cap: cap, items: make([]Item, 0, cap)}
}

func (q *queue) enqueue(it Item) bool {
	if len(q.items) >= q.cap {
		q.drops++
		if it.OnDrop != nil {
			it.OnDrop("queue_full")
		}
		return false
	}
	q.items = append(q.items, it)
	q.enq++
	q.bytes += uint64(it.Size())
	return true
}

func (q *queue) dequeue() (Item, bool) {
	if len(q.items) == 0 {
		return Item{}, false
	}
	it := q.items[0]
	q.items = q.items[1:]
	q.deq++
	if q.bytes >= uint64(it.Size()) {
		q.bytes -= uint64(it.Size())
	} else {
		q.bytes = 0
	}
	return it, true
}

func (q *queue) peekSize() int {
	if len(q.items) == 0 {
		return 0
	}
	return q.items[0].Size()
}

func (q *queue) depth() int { return len(q.items) }

type bank struct {
	mu sync.Mutex
	qs map[alf.ChannelID]*queue
}

func newBank(cfg config.QueueCap) *bank {
	return &bank{qs: map[alf.ChannelID]*queue{
		alf.ChannelSignal: newQueue(alf.ChannelSignal, cfg.Signal),
		alf.ChannelAudio:  newQueue(alf.ChannelAudio, cfg.Audio),
		alf.ChannelCursor: newQueue(alf.ChannelCursor, cfg.Cursor),
		alf.ChannelVideo:  newQueue(alf.ChannelVideo, cfg.Video),
		alf.ChannelFile:   newQueue(alf.ChannelFile, cfg.File),
	}}
}

func (b *bank) of(ch alf.ChannelID) *queue {
	if q, ok := b.qs[ch]; ok {
		return q
	}
	return b.qs[alf.ChannelFile]
}

type QueueSnap struct {
	Channel string `json:"channel"`
	Depth   int    `json:"depth"`
	Drops   uint64 `json:"drops"`
	Enqueued uint64 `json:"enqueued"`
	Dequeued uint64 `json:"dequeued"`
	Bytes   uint64 `json:"bytes"`
}

func (b *bank) Snapshot() []QueueSnap {
	b.mu.Lock()
	defer b.mu.Unlock()
	order := []alf.ChannelID{alf.ChannelSignal, alf.ChannelAudio, alf.ChannelCursor, alf.ChannelVideo, alf.ChannelFile}
	out := make([]QueueSnap, 0, len(order))
	for _, ch := range order {
		q := b.qs[ch]
		out = append(out, QueueSnap{
			Channel: ch.String(), Depth: q.depth(), Drops: q.drops,
			Enqueued: q.enq, Dequeued: q.deq, Bytes: q.bytes,
		})
	}
	return out
}
