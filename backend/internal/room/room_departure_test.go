package room_test

import (
	"testing"

	"golive/internal/alf"
	"golive/internal/room"
	"golive/internal/scheduler"
)

type recordingSink struct {
	id    string
	items []scheduler.Item
}

func (s *recordingSink) ID() string { return s.id }

func (s *recordingSink) Enqueue(item scheduler.Item) bool {
	s.items = append(s.items, item)
	return true
}

func TestFanoutContinuesAfterPeerLeaves(t *testing.T) {
	hub := room.NewHub()
	r := hub.Get("live")
	departed := &recordingSink{id: "departed"}
	remaining := &recordingSink{id: "remaining"}
	r.Join(departed)
	r.Join(remaining)

	hub.Leave("live", departed.ID())
	delivered := r.Fanout("sender", room.MediaItem(alf.ChannelVideo, 7, 42, []byte("frame")))

	if delivered != 1 {
		t.Fatalf("delivered=%d, want 1", delivered)
	}
	if len(remaining.items) != 1 || string(remaining.items[0].Payload) != "frame" {
		t.Fatalf("remaining receiver got %#v", remaining.items)
	}
}
