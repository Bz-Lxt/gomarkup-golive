package alf

import "fmt"

// Split partitions payload into ALF frames that each fit maxPayload.
// seq is shared across fragments; frag_idx / frag_total describe the set.
func Split(base Frame, payload []byte, maxPayload int) ([]Frame, error) {
	if maxPayload < 1 {
		return nil, fmt.Errorf("%w: maxPayload=%d", ErrPayloadTooBig, maxPayload)
	}
	if err := validateHeader(base, false); err != nil && base.FragTotal != 0 {
		// FragTotal may be zero on the template; we fill it.
		if !base.Channel.Valid() || !base.Kind.Valid() {
			return nil, err
		}
	}
	if !base.Channel.Valid() {
		return nil, fmt.Errorf("%w: %d", ErrBadChannel, base.Channel)
	}
	if !base.Kind.Valid() {
		return nil, fmt.Errorf("%w: %d", ErrBadKind, base.Kind)
	}
	if len(payload) == 0 {
		f := base
		f.FragIdx = 0
		f.FragTotal = 1
		f.Flags |= FlagEndOfMessage
		f.Payload = nil
		return []Frame{f}, nil
	}
	n := (len(payload) + maxPayload - 1) / maxPayload
	if n > MaxFrag {
		return nil, fmt.Errorf("%w: would need %d fragments", ErrPayloadTooBig, n)
	}
	out := make([]Frame, 0, n)
	for i := 0; i < n; i++ {
		start := i * maxPayload
		end := start + maxPayload
		if end > len(payload) {
			end = len(payload)
		}
		f := base
		f.FragIdx = uint16(i)
		f.FragTotal = uint16(n)
		f.Payload = payload[start:end]
		if i == n-1 {
			f.Flags |= FlagEndOfMessage
		} else {
			f.Flags &^= FlagEndOfMessage
		}
		out = append(out, f)
	}
	return out, nil
}
