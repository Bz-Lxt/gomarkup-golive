package room

import (
	"testing"

	"golive/internal/alf"
	"golive/internal/scheduler"
)

type stub struct {
	id string
	n  int
}

func (s *stub) ID() string { return s.id }
func (s *stub) Enqueue(scheduler.Item) bool {
	s.n++
	return true
}

func TestFanoutSkipsOrigin(t *testing.T) {
	h := NewHub()
	r := h.Get("lab")
	a, b := &stub{id: "a"}, &stub{id: "b"}
	r.Join(a)
	r.Join(b)
	n := r.Fanout("a", MediaItem(alf.ChannelCursor, 1, 0, []byte("x")))
	if n != 1 || b.n != 1 || a.n != 0 {
		t.Fatalf("n=%d a=%d b=%d", n, a.n, b.n)
	}
	if r.Size() != 2 {
		t.Fatal(r.Size())
	}
	h.Leave("lab", "a")
	if r.Size() != 1 {
		t.Fatal(r.Size())
	}
}
