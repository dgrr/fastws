package fastws

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Code to send.
type Code uint8

const (
	// CodeContinuation defines the continuation code
	CodeContinuation Code = 0x0
	// CodeText defines the text code
	CodeText Code = 0x1
	// CodeBinary defines the binary code
	CodeBinary Code = 0x2
	// CodeClose defines the close code
	CodeClose Code = 0x8
	// CodePing defines the ping code
	CodePing Code = 0x9
	// CodePong defines the pong code
	CodePong Code = 0xA
)

var zeroBytes = func() []byte {
	b := make([]byte, 10)
	for i := range b {
		b[i] = 0
	}
	return b
}()

const (
	finBit  = byte(1 << 7)
	rsv1Bit = byte(1 << 6)
	rsv2Bit = byte(1 << 5)
	rsv3Bit = byte(1 << 4)
	maskBit = byte(1 << 7)
)

// Frame is the unit used to transfer message
// between endpoints using the websocket protocol.
type Frame struct {
	max    uint64
	op     []byte
	mask   []byte
	status []byte
	b      []byte
}

// CopyTo copies the frame `fr` to `fr2`
func (fr *Frame) CopyTo(fr2 *Frame) {
	fr2.max = fr.max
	fr2.op = append(fr2.op[:0], fr.op...)
	fr2.mask = append(fr2.mask[:0], fr.mask...)
	fr2.status = append(fr2.status[:0], fr.status...)
	fr2.b = append(fr2.b[:0], fr.b...)
}

// String returns a representation of Frame in a human-readable string format.
func (fr *Frame) String() string {
	return fmt.Sprintf(`FIN: %v
RSV1: %v
RSV2: %v
RSV3: %v
--------
OPCODE: %d
--------
MASK: %v
--------
LENGTH: %d
--------
KEY: %v
--------
Data: %v`,
		fr.IsFin(), fr.HasRSV1(), fr.HasRSV2(), fr.HasRSV3(),
		fr.Code(), fr.IsMasked(), fr.Len(), fr.MaskKey(),
		fr.Payload(),
	)
}

var framePool = sync.Pool{
	New: func() interface{} {
		fr := &Frame{
			max:    DefaultPayloadSize,
			op:     make([]byte, opSize),
			mask:   make([]byte, maskSize),
			status: make([]byte, statusSize),
			b:      make([]byte, 0, 128),
		}
		return fr
	},
}

// AcquireFrame gets Frame from the global pool.
func AcquireFrame() *Frame {
	return framePool.Get().(*Frame)
}

// ReleaseFrame puts fr Frame into the global pool.
func ReleaseFrame(fr *Frame) {
	fr.Reset()
	framePool.Put(fr)
}

func (fr *Frame) resetPayload() {
	fr.b = fr.b[:0]
}

const (
	statusSize = 2
	maskSize   = 4
	opSize     = 10
)

func (fr *Frame) resetHeader() {
	copy(fr.op, zeroBytes)
	copy(fr.mask, zeroBytes)
	copy(fr.status, zeroBytes)
}

// Reset resets all Frame values to the default.
func (fr *Frame) Reset() {
	fr.resetHeader()
	fr.resetPayload()
}

// IsFin checks if FIN bit is set.
func (fr *Frame) IsFin() bool {
	return fr.op[0]&finBit != 0
}

// HasRSV1 checks if RSV1 bit is set.
func (fr *Frame) HasRSV1() bool {
	return fr.op[0]&rsv1Bit != 0
}

// HasRSV2 checks if RSV2 bit is set.
func (fr *Frame) HasRSV2() bool {
	return fr.op[0]&rsv2Bit != 0
}

// HasRSV3 checks if RSV3 bit is set.
func (fr *Frame) HasRSV3() bool {
	return fr.op[0]&rsv3Bit != 0
}

// Code returns the code set in fr.
func (fr *Frame) Code() Code {
	return Code(fr.op[0] & 15)
}

// Mode returns frame mode.
func (fr *Frame) Mode() (mode Mode) {
	switch fr.Code() {
	case CodeBinary:
		mode = ModeBinary
	default:
		mode = ModeText
	}
	return
}

// IsPing returns true if Code is CodePing.
func (fr *Frame) IsPing() bool {
	return fr.Code() == CodePing
}

// IsPong returns true if Code is CodePong.
func (fr *Frame) IsPong() bool {
	return fr.Code() == CodePong
}

// IsContinuation returns true if the Frame code is Continuation
func (fr *Frame) IsContinuation() bool {
	return fr.Code() == CodeContinuation
}

// IsClose returns true if Code is CodeClose.
func (fr *Frame) IsClose() bool {
	return fr.Code() == CodeClose
}

// IsControl ...
func (fr *Frame) IsControl() bool {
	return fr.IsClose() || fr.IsPing() || fr.IsPong()
}

// IsMasked checks if Mask bit is set.
func (fr *Frame) IsMasked() bool {
	return fr.op[1]&maskBit != 0
}

// Len returns the length of the payload based on the header bits.
//
// If you want to know the actual payload length use #PayloadLen
func (fr *Frame) Len() (length uint64) {
	length = uint64(fr.op[1] & 127)
	switch length {
	case 126:
		length = uint64(binary.BigEndian.Uint16(fr.op[2:]))
	case 127:
		length = binary.BigEndian.Uint64(fr.op[2:])
	}

	return
}

// MaskKey returns mask key.
//
// Returns zero-padded if doesn't have a mask
func (fr *Frame) MaskKey() []byte {
	return fr.mask[:4]
}

func (fr *Frame) parseStatus() {
	n := len(fr.b)
	if n >= 2 {
		// copy the status from the payload to the fr.status
		copy(fr.status, fr.b[:2])
		fr.b = append(fr.b[:0], fr.b[2:]...)
	}
}

// Payload returns the frame payload.
func (fr *Frame) Payload() []byte {
	return fr.b
}

// PayloadLen returns the actual payload length
func (fr *Frame) PayloadLen() int {
	return len(fr.b)
}

// PayloadSize returns the max payload size
func (fr *Frame) PayloadSize() uint64 {
	return fr.max
}

// SetPayloadSize sets max payload size
func (fr *Frame) SetPayloadSize(size uint64) {
	fr.max = size
}

// SetFin sets FIN bit.
func (fr *Frame) SetFin() {
	fr.op[0] |= finBit
}

// SetRSV1 sets RSV1 bit.
func (fr *Frame) SetRSV1() {
	fr.op[0] |= rsv1Bit
}

// SetRSV2 sets RSV2 bit.
func (fr *Frame) SetRSV2() {
	fr.op[0] |= rsv2Bit
}

// SetRSV3 sets RSV3 bit.
func (fr *Frame) SetRSV3() {
	fr.op[0] |= rsv3Bit
}

// SetCode sets code bits.
func (fr *Frame) SetCode(code Code) {
	// TODO: Check non-reserved fields.
	code &= 15
	fr.op[0] &= 15 << 4
	fr.op[0] |= uint8(code)
}

// SetContinuation sets CodeContinuation in Code field.
func (fr *Frame) SetContinuation() {
	fr.SetCode(CodeContinuation)
}

// SetText sets CodeText in Code field.
func (fr *Frame) SetText() {
	fr.SetCode(CodeText)
}

// SetBinary sets CodeText in Code field.
func (fr *Frame) SetBinary() {
	fr.SetCode(CodeBinary)
}

// SetClose sets CodeClose in Code field.
func (fr *Frame) SetClose() {
	fr.SetCode(CodeClose)
}

// SetPing sets CodePing in Code field.
func (fr *Frame) SetPing() {
	fr.SetCode(CodePing)
}

// SetPong sets CodePong in Code field.
func (fr *Frame) SetPong() {
	fr.SetCode(CodePong)
}

// SetMask sets the first 4 parsed bytes as mask key
// and enables the mask bit
func (fr *Frame) SetMask(b []byte) {
	fr.op[1] |= maskBit
	copy(fr.mask, b[:4])
}

// UnsetMask only drops the mask bit.
func (fr *Frame) UnsetMask() {
	fr.op[1] ^= maskBit
}

// Write appends the parsed bytes to the frame's payload
func (fr *Frame) Write(b []byte) (int, error) {
	n := len(b)
	fr.b = append(fr.b, b...)
	return n, nil
}

// SetPayload sets the parsed bytes as frame's payload
func (fr *Frame) SetPayload(b []byte) {
	fr.b = append(fr.b[:0], b...)
}

// setPayloadLen returns the number of bytes the header will use
// for sending out the payload's length.
func (fr *Frame) setPayloadLen() (s int) {
	n := len(fr.b)
	if fr.hasStatus() { // status code is embed into the payload
		n += 2
	}
	switch {
	case n > 65535:
		s = 8
		fr.setLength(127)
		binary.BigEndian.PutUint64(fr.op[2:], uint64(n))
	case n > 125:
		s = 2
		fr.setLength(126)
		binary.BigEndian.PutUint16(fr.op[2:], uint16(n))
	default:
		fr.setLength(n)
		s = 0 // assumed but ok
	}

	return
}

func (fr *Frame) setLength(n int) {
	fr.op[1] |= uint8(n)
}

// Mask performs the masking of the current payload
func (fr *Frame) Mask() {
	if len(fr.b) > 0 {
		fr.op[1] |= maskBit
		readMask(fr.mask)
		mask(fr.mask, fr.b)
	}
}

// Unmask performs the unmasking of the current payload
func (fr *Frame) Unmask() {
	if len(fr.b) > 0 {
		key := fr.MaskKey()
		mask(key, fr.b)
	}
	fr.UnsetMask()
}

func (fr *Frame) hasStatus() bool {
	return fr.status[0] > 0 || fr.status[1] > 0
}

// WriteTo writes the frame into wr.
func (fr *Frame) WriteTo(wr io.Writer) (n int64, err error) {
	var ni int
	s := fr.setPayloadLen()

	// +2 because we must include the
	// first two bytes (stuff + opcode + mask + payload len)
	ni, err = wr.Write(fr.op[:s+2])
	if err == nil {
		n += int64(ni)
		if fr.IsMasked() {
			ni, err = wr.Write(fr.mask)
			if ni > 0 {
				n += int64(ni)
			}
		}
		if err == nil {
			if fr.hasStatus() {
				ni, err = wr.Write(fr.status)
				if ni > 0 {
					n += int64(ni)
				}
			}
			if err == nil && len(fr.b) > 0 {
				ni, err = wr.Write(fr.b)
				if ni > 0 {
					n += int64(ni)
				}
			}
		}
	}

	return
}

// Status returns StatusCode.
func (fr *Frame) Status() (status StatusCode) {
	status = StatusCode(
		binary.BigEndian.Uint16(fr.status),
	)
	return
}

// SetStatus sets status code.
//
// Status code is usually used in Close request.
func (fr *Frame) SetStatus(status StatusCode) {
	binary.BigEndian.PutUint16(fr.status, uint16(status))
}

// mustRead returns the number of bytes that must be
// read to decode the length of the payload.
func (fr *Frame) mustRead() (n int) {
	n = int(fr.op[1] & 127)
	switch n {
	case 127:
		n = 8
	case 126:
		n = 2
	default:
		n = 0
	}

	return
}

var (
	// EOF represents an io.EOF error.
	EOF                = io.EOF
	errMalformedHeader = errors.New("Malformed header")
	errBadHeaderSize   = errors.New("Header size is insufficient")
)

// ReadFrom fills fr reading from rd.
func (fr *Frame) ReadFrom(rd io.Reader) (int64, error) {
	return fr.readFrom(rd)
}

var (
	errReadingHeader = errors.New("error reading frame header")
	errReadingLen    = errors.New("error reading b length")
	errReadingMask   = errors.New("error reading mask")
	errLenTooBig     = errors.New("message length is bigger than expected")
	errStatusLen     = errors.New("length of the status must be = 2")
)

const limitLen = 1 << 32

func (fr *Frame) readFrom(r io.Reader) (int64, error) {
	var err error
	var n, m int

	// read the first 2 bytes (stuff + opcode + maskbit + payload len)
	n, err = io.ReadFull(r, fr.op[:2])
	if err == io.ErrUnexpectedEOF {
		err = errReadingHeader
	}
	if err == nil {
		// get how many bytes we should read to read the length
		m = fr.mustRead() + 2
		if m > 2 { // reading length
			n, err = io.ReadFull(r, fr.op[2:m]) // start from 2 to fill in 2:m
			if err == io.ErrUnexpectedEOF {
				err = errReadingLen
			}
		}

		if err == nil && fr.IsMasked() { // reading mask
			n, err = io.ReadFull(r, fr.mask[:4])
			if err == io.ErrUnexpectedEOF {
				err = errReadingMask
			}
		}

		if err == nil {
			// reading the payload
			fr.op[2] &= 127 // quick fix to prevent overflow
			if nn := fr.Len(); (fr.max > 0 && nn > fr.max) || nn > limitLen {
				err = errLenTooBig
			} else if nn > 0 {
				isClose := fr.IsClose()
				if isClose {
					nn -= 2
					if nn < 0 {
						err = errStatusLen
					}
				}

				if err == nil {
					if rLen := int64(nn) - int64(cap(fr.b)); rLen > 0 {
						fr.b = append(fr.b[:cap(fr.b)], make([]byte, rLen)...)
					}

					if isClose {
						n, err = io.ReadFull(r, fr.status[:2])
						if err == io.ErrUnexpectedEOF {
							err = errStatusLen
						}
					}

					if err == nil {
						fr.b = fr.b[:nn]
						n, err = io.ReadFull(r, fr.b)
					}
				}
			}
		}
	}

	return int64(n), err
}
