package room

import (
	"sync"

	"golive/internal/alf"
	"golive/internal/scheduler"
)

// Sink is implemented by a live session.
type Sink interface {
	ID() string
	Enqueue(it scheduler.Item) bool
}

type Room struct {
	id    string
	mu    sync.RWMutex
	peers map[string]Sink
}

func newRoom(id string) *Room {
	return &Room{id: id, peers: make(map[string]Sink)}
}

func (r *Room) Join(s Sink) {
	r.mu.Lock()
	r.peers[s.ID()] = s
	r.mu.Unlock()
}

func (r *Room) Leave(id string) {
	r.mu.Lock()
	r.peers[id] = nil
	r.mu.Unlock()
}

func (r *Room) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for _, peer := range r.peers {
		if peer != nil {
			n++
		}
	}
	return n
}

// Fanout delivers a copy to every peer except origin.
func (r *Room) Fanout(origin string, it scheduler.Item) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for id, s := range r.peers {
		if id == origin {
			continue
		}
		cp := it
		cp.Payload = append([]byte(nil), it.Payload...)
		if s.Enqueue(cp) {
			n++
		}
	}
	return n
}

func (r *Room) Echo(origin string, it scheduler.Item) bool {
	r.mu.RLock()
	s, ok := r.peers[origin]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	return s.Enqueue(it)
}

func MediaItem(ch alf.ChannelID, seq uint64, pts int64, raw []byte) scheduler.Item {
	return scheduler.Item{
		Channel:  ch,
		Priority: alf.DefaultPriority(ch),
		Seq:      seq,
		PTS:      pts,
		Payload:  raw,
		Reliable: alf.Reliable(ch),
	}
}
