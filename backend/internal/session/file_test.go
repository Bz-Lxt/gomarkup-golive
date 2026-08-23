package session

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"golive/internal/signal"
)

func TestFileRoundTripHash(t *testing.T) {
	payload := []byte("weak-net-file-body-0123456789")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])
	s := newFileSink()
	if err := s.Begin(signal.FileBegin{Name: "a.bin", Size: int64(len(payload)), SHA256: hexSum}); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(payload[:10]); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(payload[10:]); err != nil {
		t.Fatal(err)
	}
	ack := s.Finish()
	if !ack.Match {
		t.Fatalf("ack=%+v", ack)
	}
}

func TestFileRejectsHuge(t *testing.T) {
	s := newFileSink()
	if err := s.Begin(signal.FileBegin{Name: "x", Size: maxFileBytes + 1}); err == nil {
		t.Fatal("expected size error")
	}
}

func TestFileChunkBeforeBegin(t *testing.T) {
	s := newFileSink()
	if err := s.Write([]byte("x")); err == nil {
		t.Fatal("expected error")
	}
}
