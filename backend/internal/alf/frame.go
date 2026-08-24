package alf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

const (
	Magic     uint32 = 0x474C4146 // "GLAF"
	Version   uint8  = 1
	HeaderSize       = 35
	MaxFrag          = 4096
)

// Flags bitset.
const (
	FlagEndOfMessage uint8 = 1 << 0
	FlagAck          uint8 = 1 << 1
	FlagPing         uint8 = 1 << 2
	FlagPong         uint8 = 1 << 3
	FlagReset        uint8 = 1 << 4
)

var (
	ErrShortHeader   = errors.New("alf: short header")
	ErrBadMagic      = errors.New("alf: bad magic")
	ErrBadVersion    = errors.New("alf: unsupported version")
	ErrBadChannel    = errors.New("alf: invalid channel")
	ErrBadKind       = errors.New("alf: invalid stream kind")
	ErrBadFrag       = errors.New("alf: invalid fragment index")
	ErrLenMismatch   = errors.New("alf: payload length mismatch")
	ErrChecksum      = errors.New("alf: checksum mismatch")
	ErrPayloadTooBig = errors.New("alf: payload exceeds limit")
	ErrTruncated     = errors.New("alf: truncated frame")
)

// Frame is the application-layer unit carried over WebTransport streams/datagrams.
//
// Binary layout (big-endian), 35-byte header:
//
//	 0- 3  magic
//	 4     version
//	 5     channel_id
//	 6     stream_kind
//	 7     priority
//	 8-15  seq
//	16-23  pts (unix ms, Beijing-derived clock)
//	24     flags
//	25-26  frag_idx
//	27-28  frag_total
//	29-30  payload_len
//	31-34  crc32 (IEEE, over header[0:31] + payload)
//	35-    payload
type Frame struct {
	Channel   ChannelID
	Kind      Kind
	Priority  Priority
	Seq       uint64
	PTS       int64
	Flags     uint8
	FragIdx   uint16
	FragTotal uint16
	Payload   []byte
}

func (f Frame) EndOfMessage() bool {
	return f.Flags&FlagEndOfMessage != 0 || f.FragTotal <= 1 || f.FragIdx+1 == f.FragTotal
}

func (f Frame) Clone() Frame {
	out := f
	if f.Payload != nil {
		out.Payload = append([]byte(nil), f.Payload...)
	}
	return out
}

func (f Frame) Size() int {
	return HeaderSize + len(f.Payload)
}

func Encode(f Frame) ([]byte, error) {
	if err := validateHeader(f, true); err != nil {
		return nil, err
	}
	if len(f.Payload) > 0xFFFF {
		return nil, fmt.Errorf("%w: %d", ErrPayloadTooBig, len(f.Payload))
	}
	buf := make([]byte, HeaderSize+len(f.Payload))
	if err := writeHeader(buf, f); err != nil {
		return nil, err
	}
	copy(buf[HeaderSize:], f.Payload)
	crc := crc32.ChecksumIEEE(buf[:31])
	crc = crc32.Update(crc, crc32.IEEETable, f.Payload)
	binary.BigEndian.PutUint32(buf[31:35], crc)
	return buf, nil
}

func Decode(raw []byte) (Frame, error) {
	var zero Frame
	if len(raw) < HeaderSize {
		return zero, ErrShortHeader
	}
	if binary.BigEndian.Uint32(raw[0:4]) != Magic {
		return zero, ErrBadMagic
	}
	if raw[4] != Version {
		return zero, ErrBadVersion
	}
	f := Frame{
		Channel:   ChannelID(raw[5]),
		Kind:      Kind(raw[6]),
		Priority:  Priority(raw[7]),
		Seq:       binary.BigEndian.Uint64(raw[8:16]),
		PTS:       int64(binary.BigEndian.Uint64(raw[16:24])),
		Flags:     raw[24],
		FragIdx:   binary.BigEndian.Uint16(raw[25:27]),
		FragTotal: binary.BigEndian.Uint16(raw[27:29]),
	}
	plen := int(binary.BigEndian.Uint16(raw[29:31]))
	if plen < 0 || HeaderSize+plen > len(raw) {
		return zero, ErrTruncated
	}
	if HeaderSize+plen != len(raw) {
		return zero, ErrLenMismatch
	}
	if err := validateHeader(f, false); err != nil {
		return zero, err
	}
	want := binary.BigEndian.Uint32(raw[31:35])
	crc := crc32.ChecksumIEEE(raw[:31])
	crc = crc32.Update(crc, crc32.IEEETable, raw[HeaderSize:HeaderSize+plen])
	if crc != want {
		return zero, ErrChecksum
	}
	if plen > 0 {
		f.Payload = append([]byte(nil), raw[HeaderSize:HeaderSize+plen]...)
	}
	return f, nil
}

func writeHeader(buf []byte, f Frame) error {
	if len(buf) < HeaderSize {
		return ErrShortHeader
	}
	binary.BigEndian.PutUint32(buf[0:4], Magic)
	buf[4] = Version
	buf[5] = uint8(f.Channel)
	buf[6] = uint8(f.Kind)
	buf[7] = uint8(f.Priority)
	binary.BigEndian.PutUint64(buf[8:16], f.Seq)
	binary.BigEndian.PutUint64(buf[16:24], uint64(f.PTS))
	buf[24] = f.Flags
	binary.BigEndian.PutUint16(buf[25:27], f.FragIdx)
	binary.BigEndian.PutUint16(buf[27:29], f.FragTotal)
	binary.BigEndian.PutUint16(buf[29:31], uint16(len(f.Payload)))
	return nil
}

func validateHeader(f Frame, encoding bool) error {
	if !f.Channel.Valid() {
		return fmt.Errorf("%w: %d", ErrBadChannel, f.Channel)
	}
	if !f.Kind.Valid() {
		return fmt.Errorf("%w: %d", ErrBadKind, f.Kind)
	}
	if f.FragTotal == 0 {
		return fmt.Errorf("%w: frag_total=0", ErrBadFrag)
	}
	if f.FragTotal > MaxFrag {
		return fmt.Errorf("%w: frag_total=%d", ErrBadFrag, f.FragTotal)
	}
	if f.FragIdx >= f.FragTotal {
		return fmt.Errorf("%w: idx=%d total=%d", ErrBadFrag, f.FragIdx, f.FragTotal)
	}
	if encoding && f.Payload == nil {
		f.Payload = []byte{}
	}
	return nil
}

// MaxPayloadForDatagram returns the largest payload that still fits in a
// Chrome-capped datagram after the ALF header.
func MaxPayloadForDatagram(maxDatagram int) int {
	if maxDatagram <= 0 {
		maxDatagram = 1024
	}
	if maxDatagram > 1024 {
		maxDatagram = 1024
	}
	n := maxDatagram - HeaderSize
	if n < 1 {
		return 0
	}
	return n
}
