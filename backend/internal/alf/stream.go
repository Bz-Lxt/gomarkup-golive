package alf

import (
	"encoding/binary"
	"fmt"
	"io"
)

// StreamWriter writes length-prefixed ALF frames onto a reliable stream.
// Prefix is a 4-byte big-endian length of the encoded frame (including header).
type StreamWriter struct {
	w io.Writer
}

func NewStreamWriter(w io.Writer) *StreamWriter {
	return &StreamWriter{w: w}
}

func (sw *StreamWriter) WriteFrame(f Frame) error {
	raw, err := Encode(f)
	if err != nil {
		return err
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(raw)))
	if _, err := sw.w.Write(prefix[:]); err != nil {
		return fmt.Errorf("alf stream prefix: %w", err)
	}
	if _, err := sw.w.Write(raw); err != nil {
		return fmt.Errorf("alf stream body: %w", err)
	}
	return nil
}

type StreamReader struct {
	r       io.Reader
	maxSize int
}

func NewStreamReader(r io.Reader, maxSize int) *StreamReader {
	if maxSize <= 0 {
		maxSize = HeaderSize + (1 << 20)
	}
	return &StreamReader{r: r, maxSize: maxSize}
}

func (sr *StreamReader) ReadFrame() (Frame, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(sr.r, prefix[:]); err != nil {
		return Frame{}, err
	}
	n := int(binary.BigEndian.Uint32(prefix[:]))
	if n < HeaderSize || n > sr.maxSize {
		return Frame{}, fmt.Errorf("%w: framed size %d", ErrPayloadTooBig, n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(sr.r, buf); err != nil {
		if err == io.EOF {
			return Frame{}, ErrTruncated
		}
		return Frame{}, err
	}
	return Decode(buf)
}
