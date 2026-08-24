package session

import (
	"sync"
	"time"
)

type Registry struct {
	mu    sync.RWMutex
	items map[string]*Session
}

func NewRegistry() *Registry {
	return &Registry{items: make(map[string]*Session)}
}

func (r *Registry) Add(s *Session) {
	r.mu.Lock()
	r.items[s.id] = s
	r.mu.Unlock()
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	delete(r.items, id)
	r.mu.Unlock()
}

func (r *Registry) Get(id string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.items[id]
	return s, ok
}

func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}

func (r *Registry) ReapIdle(idle time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	now := time.Now()
	for id, s := range r.items {
		if now.Sub(s.LastActive()) > idle {
			s.Close("idle")
			delete(r.items, id)
			n++
		}
	}
	return n
}

func (r *Registry) Snapshot() []Info {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Info, 0, len(r.items))
	for _, s := range r.items {
		out = append(out, s.Info())
	}
	return out
}
