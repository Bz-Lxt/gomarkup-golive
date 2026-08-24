package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"golive/internal/signal"
)

const maxFileBytes = 32 << 20 // 32 MiB cap for the experiment bench

type fileSink struct {
	mu       sync.Mutex
	meta     signal.FileBegin
	buf      []byte
	hasher   sha256Hash
	started  bool
}

type sha256Hash = interface {
	Write(p []byte) (int, error)
	Sum(b []byte) []byte
}

func newFileSink() *fileSink {
	return &fileSink{}
}

func (f *fileSink) Begin(m signal.FileBegin) error {
	if m.Size < 0 || m.Size > maxFileBytes {
		return fmt.Errorf("file size %d out of range", m.Size)
	}
	if m.Name == "" {
		return fmt.Errorf("file name required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.meta = m
	f.buf = make([]byte, 0, min64(m.Size, 1<<20))
	f.hasher = sha256.New()
	f.started = true
	return nil
}

func (f *fileSink) Write(p []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.started {
		return fmt.Errorf("file chunk before begin")
	}
	if int64(len(f.buf)+len(p)) > maxFileBytes {
		return fmt.Errorf("file exceeds cap")
	}
	f.buf = append(f.buf, p...)
	_, _ = f.hasher.Write(p)
	return nil
}

func (f *fileSink) Finish() signal.FileAck {
	f.mu.Lock()
	defer f.mu.Unlock()
	ack := signal.FileAck{Name: f.meta.Name, Received: int64(len(f.buf))}
	if !f.started {
		ack.Error = "no transfer in progress"
		return ack
	}
	sum := hex.EncodeToString(f.hasher.Sum(nil))
	ack.SHA256 = sum
	ack.Match = sum == f.meta.SHA256 && (f.meta.Size == 0 || ack.Received == f.meta.Size)
	if !ack.Match && f.meta.SHA256 != "" {
		ack.Error = "sha256 mismatch"
	}
	f.started = false
	f.buf = nil
	return ack
}

func min64(a, b int64) int64 {
	if a < b && a > 0 {
		return a
	}
	return b
}
