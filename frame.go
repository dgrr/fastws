package fastws

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
	maxHeaderSize  = 14
	maxPayloadSize = uint64(4096)
)

// Code to send.
type Code uint8

const (
	CodeContinuation Code = 0x0
	CodeText         Code = 0x1
	CodeBinary       Code = 0x2
	CodeClose        Code = 0x8
	CodePing         Code = 0x9
	CodePong         Code = 0xA
)

var zeroBytes = func() []byte {
	b := make([]byte, maxHeaderSize)
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
// between endpoints using websocket protocol.
//
// Frame could not be used during message exchanging.
// This type can be used if you want low level access to websocket.
type Frame struct {
	max uint64
	b   []byte
}

var framePool = sync.Pool{
	New: func() interface{} {
		fr := &Frame{
			max: maxPayloadSize,
			b:   make([]byte, maxHeaderSize, 128),
		}
		return fr
	},
}

// AcquireFrame gets Frame from pool.
func AcquireFrame() *Frame {
	return framePool.Get().(*Frame)
}

// ReleaseFrame puts fr Frame into the pool.
func ReleaseFrame(fr *Frame) {
	fr.Reset()
	framePool.Put(fr)
}

// Reset resets all Frame values to be reused.
func (fr *Frame) Reset() {
	fr.b = fr.b[:maxHeaderSize]
	copy(fr.b, zeroBytes)
}

// IsFin checks if FIN bit is set.
func (fr *Frame) IsFin() bool {
	return fr.b[0]&finBit != 0
}

// HasRSV1 checks if RSV1 bit is set.
func (fr *Frame) HasRSV1() bool {
	return fr.b[0]&rsv1Bit != 0
}

// HasRSV2 checks if RSV2 bit is set.
func (fr *Frame) HasRSV2() bool {
	return fr.b[0]&rsv2Bit != 0
}

// HasRSV3 checks if RSV3 bit is set.
func (fr *Frame) HasRSV3() bool {
	return fr.b[0]&rsv3Bit != 0
}

// Code returns the code set in fr.
func (fr *Frame) Code() Code {
	return Code(fr.b[0] & 15)
}

// Mode returns frame mode.
func (fr *Frame) Mode() (mode Mode) {
	switch fr.Code() {
	case CodeText:
		mode = ModeText
	case CodeBinary:
		mode = ModeBinary
	default:
		mode = ModeNone
	}
	return
}

// IsPong returns true if Code is CodePing.
func (fr *Frame) IsPing() bool {
	return fr.Code() == CodePing
}

// IsPong returns true if Code is CodePong.
func (fr *Frame) IsPong() bool {
	return fr.Code() == CodePong
}

func (fr *Frame) IsContinuation() bool {
	return fr.Code() == CodeContinuation
}

// IsClose returns true if Code is CodeClose.
func (fr *Frame) IsClose() bool {
	return fr.Code() == CodeClose
}

// IsMasked checks if Mask bit is set.
func (fr *Frame) IsMasked() bool {
	return fr.b[1]&maskBit != 0
}

// Len returns payload length based on Frame field of length bytes.
func (fr *Frame) Len() (length uint64) {
	length = uint64(fr.b[1] & 127)
	switch length {
	case 126:
		length = uint64(binary.BigEndian.Uint16(fr.b[2:]))
	case 127:
		length = binary.BigEndian.Uint64(fr.b[2:])
	}
	return length
}

// MaskKey returns mask key if exist.
func (fr *Frame) MaskKey() []byte {
	return fr.b[10:14]
}

// Payload returns Frame payload.
func (fr *Frame) Payload() []byte {
	n := maxHeaderSize
	if fr.IsClose() {
		n += 2
	}
	return fr.b[n:]
}

// MaxPayloadSize returns max payload size.
func (fr *Frame) MaxPayloadSize() uint64 {
	return fr.max
}

// SetMaxPayloadSize sets max payload size.
func (fr *Frame) SetMaxPayloadSize(size uint64) {
	fr.max = size
}

// SetFin sets FIN bit.
func (fr *Frame) SetFin() {
	fr.b[0] |= finBit
}

// SetRSV1 sets RSV1 bit.
func (fr *Frame) SetRSV1() {
	fr.b[0] |= rsv1Bit
}

// SetRSV2 sets RSV2 bit.
func (fr *Frame) SetRSV2() {
	fr.b[0] |= rsv2Bit
}

// SetRSV3 sets RSV3 bit.
func (fr *Frame) SetRSV3() {
	fr.b[0] |= rsv3Bit
}

// SetCode sets code bits.
func (fr *Frame) SetCode(code Code) {
	// TODO: Check non-reserved fields.
	code &= 15
	fr.b[0] &= 15 << 4
	fr.b[0] |= uint8(code)
}

// SetContinuation sets CodeContinuation in Code field.
func (fr *Frame) SetContinuation() {
	fr.SetCode(CodeContinuation)
}

// SetText sets CodeText in Code field.
func (fr *Frame) SetText() {
	fr.SetCode(CodeText)
}

// SetText sets CodeText in Code field.
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

// SetMask sets mask key to mask the frame and enabled mask bit.
func (fr *Frame) SetMask(b []byte) {
	// TODO: Bound checking?
	fr.b[1] |= maskBit
	copy(fr.b[10:14], b[:4])
}

// UnsetMask drops mask bit.
func (fr *Frame) UnsetMask() {
	fr.b[1] ^= maskBit
}

// Write writes b to the frame payload.
func (fr *Frame) Write(b []byte) (int, error) {
	n := len(b)
	fr.b = append(fr.b, b...)
	return n, nil
}

// SetPayload sets payload to fr.
func (fr *Frame) SetPayload(b []byte) {
	n := maxHeaderSize
	if fr.IsClose() {
		n += 2
	}
	fr.b = append(fr.b[:n], b...)
}

func (fr *Frame) setPayloadLen() (s int) {
	n := len(fr.b[maxHeaderSize:])
	switch {
	case n > 65535:
		s = 8
		fr.b[1] |= uint8(127)
		binary.BigEndian.PutUint64(fr.b[2:], uint64(n))
	case n > 125:
		s = 2
		fr.b[1] |= uint8(126)
		binary.BigEndian.PutUint16(fr.b[2:], uint16(n))
	default:
		s = 0
		fr.b[1] |= uint8(n)
	}
	return
}

// Mask masks Frame payload.
func (fr *Frame) Mask() {
	fr.b[1] |= maskBit
	readMask(fr.b[10:14])
	mask(fr.b[10:14], fr.b[maxHeaderSize:])
}

// Unmask unmasks Frame payload.
func (fr *Frame) Unmask() {
	key := fr.MaskKey()
	if len(key) == 4 {
		mask(key, fr.b[maxHeaderSize:])
		fr.UnsetMask()
	}
}

// WriteTo marshals the frame and writes the frame into wr.
func (fr *Frame) WriteTo(wr io.Writer) (nn int64, err error) {
	n := fr.setPayloadLen() + 2
	n, err = wr.Write(fr.b[:n]) // writing first 2 bytes + length
	if err == nil {
		nn += int64(n)
		if fr.IsMasked() {
			n, err = wr.Write(fr.b[10:14]) // writing mask
			nn += int64(n)
		}
		if err == nil {
			n, err = wr.Write(fr.b[maxHeaderSize:]) // writing payload
			nn += int64(n)
		}
	}
	return nn, err
}

// Status returns StatusCode from request payload.
//
// The frame code should be CodeClose.
//
// If the code was not found or the frame code is not CodeClose then it will return StatusUndefined.
func (fr *Frame) Status() (status StatusCode) {
	if fr.IsClose() {
		status = StatusCode(
			binary.BigEndian.Uint16(fr.b[maxHeaderSize : maxHeaderSize+2]),
		)
	}
	return
}

// SetStatus sets status code to the CodeClose request/response.
//
// If the Frame Code is not CodeClose the status won't be set.
func (fr *Frame) SetStatus(status StatusCode) {
	if fr.IsClose() {
		if cap(fr.b) < maxHeaderSize+2 {
			fr.b = append(fr.b[:cap(fr.b)], make([]byte, 2)...)
		}
		fr.b = fr.b[:maxHeaderSize+2]

		binary.BigEndian.PutUint16(fr.b[maxHeaderSize:], uint16(status))
	}
}

// mustRead returns the bytes to be readed to decode the length
// of the payload.
func (fr *Frame) mustRead() (n int) {
	n = int(fr.b[1] & 127)
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
	EOF                = io.EOF
	errMalformedHeader = errors.New("Malformed header")
	errBadHeaderSize   = errors.New("Header size is insufficient")
)

// ReadFrom fills fr reading from rd.
//
// if rd == nil then ReadFrom returns EOF
func (fr *Frame) ReadFrom(rd io.Reader) (nn int64, err error) {
	if rd == nil {
		err = EOF
	} else {
		nn, err = fr.readFrom(rd)
	}
	return
}

var (
	errReadingHeader = errors.New("error reading frame header")
	errReadingLen    = errors.New("error reading payload length")
	errReadingMask   = errors.New("error reading mask")
)

func (fr *Frame) readFrom(br io.Reader) (int64, error) {
	var n, m int

	n, err := br.Read(fr.b[:2])
	if err == nil && n < 2 {
		err = errReadingHeader
	}
	if err == nil {
		m = fr.mustRead() + 2
		if m > 2 { // reading length
			n, err = br.Read(fr.b[2:m])
			if n+2 < m {
				err = errReadingLen
			}
		}
		if err == nil && fr.IsMasked() { // reading mask
			m, err = br.Read(fr.b[10:14])
			if m < 4 {
				err = errReadingMask
			}
		}
		if err == nil { // reading payload
			if nn := fr.Len(); fr.max > 0 && nn > fr.max {
				err = fmt.Errorf("Max payload size exceeded (%d < %d)", fr.max, fr.Len())
			} else if nn > 0 {
				// TODO: prevent int64(1<<64 - 1) conversion
				if rLen := maxHeaderSize + int64(nn) - int64(cap(fr.b)); rLen > 0 {
					fr.b = append(fr.b[:cap(fr.b)], make([]byte, rLen)...)
				}

				n, err = br.Read(fr.b[maxHeaderSize : nn+maxHeaderSize])
				if err == nil {
					fr.b = fr.b[:n+maxHeaderSize]
				}
			}
		}
	}
	return int64(n), err
}
