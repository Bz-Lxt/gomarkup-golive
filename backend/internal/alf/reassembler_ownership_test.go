package alf_test

import (
	"bytes"
	"testing"
	"time"

	"golive/internal/alf"
)

func TestReassemblerRetainsAcceptedFragmentBytes(t *testing.T) {
	r := alf.NewReassembler(time.Second, 1024)
	now := time.Unix(1_700_000_000, 0)
	scratch := make([]byte, 4)
	copy(scratch, "head")

	first := alf.Frame{
		Channel:   alf.ChannelCursor,
		Kind:      alf.KindDatagram,
		Priority:  alf.PrioCursor,
		Seq:       73,
		FragIdx:   0,
		FragTotal: 2,
		Payload:   scratch,
	}
	if _, ok, err := r.Push(first, now); err != nil || ok {
		t.Fatalf("first fragment: ok=%v err=%v", ok, err)
	}

	copy(scratch, "tail")
	last := first
	last.Flags = alf.FlagEndOfMessage
	last.FragIdx = 1
	got, ok, err := r.Push(last, now)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("message did not complete")
	}
	if want := []byte("headtail"); !bytes.Equal(got.Payload, want) {
		t.Fatalf("reassembled payload = %q, want %q", got.Payload, want)
	}
}
