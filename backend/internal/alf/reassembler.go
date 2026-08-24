package alf

import (
	"sync"
	"time"
)

type key struct {
	ch  ChannelID
	seq uint64
}

type pending struct {
	kind      Kind
	prio      Priority
	pts       int64
	flags     uint8
	total     uint16
	parts     map[uint16][]byte
	deadline  time.Time
}

// Reassembler collects fragments. Unreliable channels drop a message when
// the deadline elapses. Reliable channels keep waiting until Complete or Reset.
type Reassembler struct {
	mu      sync.Mutex
	ttl     time.Duration
	maxPay  int
	items   map[key]*pending
	dropped uint64
}

type Complete struct {
	Frame Frame
}

func NewReassembler(ttl time.Duration, maxPayload int) *Reassembler {
	if ttl <= 0 {
		ttl = 200 * time.Millisecond
	}
	if maxPayload <= 0 {
		maxPayload = 1 << 20
	}
	return &Reassembler{
		ttl:    ttl,
		maxPay: maxPayload,
		items:  make(map[key]*pending),
	}
}

func (r *Reassembler) Dropped() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dropped
}

func (r *Reassembler) Inflight() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.items)
}

// Push inserts one fragment. A completed message is returned; incomplete
// returns ok=false. Malformed fragments return an error and are not stored.
func (r *Reassembler) Push(f Frame, now time.Time) (complete Frame, ok bool, err error) {
	if err := validateHeader(f, false); err != nil {
		return Frame{}, false, err
	}
	if len(f.Payload) > r.maxPay {
		return Frame{}, false, ErrPayloadTooBig
	}
	if f.FragTotal == 1 {
		r.mu.Lock()
		delete(r.items, key{f.Channel, f.Seq})
		r.mu.Unlock()
		return f, true, nil
	}

	k := key{f.Channel, f.Seq}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(now)

	p, exists := r.items[k]
	if !exists {
		p = &pending{
			kind:     f.Kind,
			prio:     f.Priority,
			pts:      f.PTS,
			flags:    f.Flags,
			total:    f.FragTotal,
			parts:    make(map[uint16][]byte, f.FragTotal),
			deadline: now.Add(r.ttl),
		}
		r.items[k] = p
	}
	if p.total != f.FragTotal {
		delete(r.items, k)
		r.dropped++
		return Frame{}, false, ErrBadFrag
	}
	if _, dup := p.parts[f.FragIdx]; !dup {
		p.parts[f.FragIdx] = append([]byte(nil), f.Payload...)
	}
	if uint16(len(p.parts)) < p.total {
		return Frame{}, false, nil
	}
	totalLen := 0
	for i := uint16(0); i < p.total; i++ {
		part, have := p.parts[i]
		if !have {
			return Frame{}, false, nil
		}
		totalLen += len(part)
		if totalLen > r.maxPay {
			delete(r.items, k)
			return Frame{}, false, ErrPayloadTooBig
		}
	}
	payload := make([]byte, 0, totalLen)
	r.mu.Unlock()
	for i := uint16(0); i < p.total; i++ {
		payload = append(payload, p.parts[i]...)
	}
	r.mu.Lock()
	delete(r.items, k)
	out := Frame{
		Channel:   f.Channel,
		Kind:      p.kind,
		Priority:  p.prio,
		Seq:       f.Seq,
		PTS:       p.pts,
		Flags:     p.flags | FlagEndOfMessage,
		FragIdx:   0,
		FragTotal: 1,
		Payload:   payload,
	}
	return out, true, nil
}

// Sweep drops expired unreliable messages. Reliable channels are not swept
// by TTL (they wait for the session to close).
func (r *Reassembler) Sweep(now time.Time) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.expireLocked(now)
}

func (r *Reassembler) expireLocked(now time.Time) uint64 {
	var n uint64
	for k, p := range r.items {
		if !Reliable(k.ch) && now.After(p.deadline) {
			delete(r.items, k)
			n++
		}
	}
	r.dropped += n
	return n
}

func (r *Reassembler) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = make(map[key]*pending)
}
