package alf

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func sample(payload []byte) Frame {
	return Frame{
		Channel:   ChannelCursor,
		Kind:      KindDatagram,
		Priority:  PrioCursor,
		Seq:       42,
		PTS:       1_725_000_000_000,
		Flags:     FlagEndOfMessage,
		FragIdx:   0,
		FragTotal: 1,
		Payload:   payload,
	}
}

func TestRoundTrip(t *testing.T) {
	orig := sample([]byte("hello-golive"))
	raw, err := Encode(orig)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != HeaderSize+len(orig.Payload) {
		t.Fatalf("size=%d", len(raw))
	}
	got, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Channel != orig.Channel || got.Seq != orig.Seq || !bytes.Equal(got.Payload, orig.Payload) {
		t.Fatalf("mismatch %+v vs %+v", got, orig)
	}
}

func TestDecodeTable(t *testing.T) {
	good, _ := Encode(sample([]byte{1, 2, 3}))
	tests := []struct {
		name string
		in   []byte
		want error
	}{
		{"short", []byte{1, 2, 3}, ErrShortHeader},
		{"bad magic", mutate(good, 0, 0x00), ErrBadMagic},
		{"bad version", mutate(good, 4, 9), ErrBadVersion},
		{"truncated", good[:HeaderSize+1], ErrTruncated},
		{"checksum", mutate(good, HeaderSize, 0xFF), ErrChecksum},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(tc.in)
			if err == nil {
				t.Fatal("expected error")
			}
			if tc.want != nil && err != tc.want && !containsErr(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}

func TestInvalidChannelRejected(t *testing.T) {
	f := sample([]byte("x"))
	f.Channel = 99
	if _, err := Encode(f); err == nil {
		t.Fatal("expected bad channel")
	}
}

func TestInvalidFragRejected(t *testing.T) {
	f := sample([]byte("x"))
	f.FragTotal = 0
	if _, err := Encode(f); err == nil {
		t.Fatal("expected bad frag")
	}
	f.FragTotal = 2
	f.FragIdx = 2
	if _, err := Encode(f); err == nil {
		t.Fatal("expected idx>=total")
	}
}

func TestChecksumCoversPayload(t *testing.T) {
	raw, _ := Encode(sample([]byte{9, 8, 7}))
	crcStored := binary.BigEndian.Uint32(raw[31:35])
	crc := crc32.ChecksumIEEE(raw[:31])
	crc = crc32.Update(crc, crc32.IEEETable, raw[HeaderSize:])
	if crc != crcStored {
		t.Fatalf("self crc mismatch")
	}
}

func TestSplitAndReassemble(t *testing.T) {
	payload := bytes.Repeat([]byte("abcd"), 80) // 320 bytes
	base := sample(nil)
	base.Flags = 0
	frags, err := Split(base, payload, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 7 {
		t.Fatalf("frags=%d", len(frags))
	}
	r := NewReassembler(0, 1<<16)
	var done Frame
	var ok bool
	// feed out of order
	order := []int{3, 0, 6, 1, 5, 2, 4}
	for _, i := range order {
		done, ok, err = r.Push(frags[i], now())
		if err != nil {
			t.Fatal(err)
		}
	}
	if !ok {
		t.Fatal("expected complete")
	}
	if !bytes.Equal(done.Payload, payload) {
		t.Fatalf("payload mismatch len=%d", len(done.Payload))
	}
}

func TestUnreliableTimeoutDrop(t *testing.T) {
	base := sample(nil)
	base.Channel = ChannelAudio
	base.Flags = 0
	frags, err := Split(base, bytes.Repeat([]byte{1}, 40), 20)
	if err != nil {
		t.Fatal(err)
	}
	r := NewReassembler(5, 1<<16)
	t0 := now()
	if _, ok, err := r.Push(frags[0], t0); err != nil || ok {
		t.Fatalf("first frag ok=%v err=%v", ok, err)
	}
	n := r.Sweep(t0.Add(10))
	if n != 1 {
		t.Fatalf("dropped=%d", n)
	}
	if r.Dropped() == 0 {
		t.Fatal("counter")
	}
}

func TestReliableNotSwept(t *testing.T) {
	base := sample(nil)
	base.Channel = ChannelFile
	base.Kind = KindBidi
	base.Flags = 0
	frags, _ := Split(base, bytes.Repeat([]byte{2}, 40), 20)
	r := NewReassembler(5, 1<<16)
	t0 := now()
	_, _, _ = r.Push(frags[0], t0)
	if n := r.Sweep(t0.Add(10)); n != 0 {
		t.Fatalf("reliable should not expire, n=%d", n)
	}
}

func TestStreamFraming(t *testing.T) {
	var buf bytes.Buffer
	w := NewStreamWriter(&buf)
	f := sample([]byte("stream-body"))
	if err := w.WriteFrame(f); err != nil {
		t.Fatal(err)
	}
	r := NewStreamReader(&buf, 4096)
	got, err := r.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Payload, f.Payload) {
		t.Fatal("stream payload")
	}
}

func TestMaxPayloadForDatagramCaps1024(t *testing.T) {
	if got := MaxPayloadForDatagram(2048); got != 1024-HeaderSize {
		t.Fatalf("got %d", got)
	}
}

func mutate(in []byte, idx int, v byte) []byte {
	out := append([]byte(nil), in...)
	out[idx] = v
	return out
}

func containsErr(err, want error) bool {
	return err == want || (err != nil && want != nil && (err.Error() == want.Error()))
}
