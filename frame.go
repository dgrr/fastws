package fastws

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
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
// between endpoints using websocket protocol.
//
// Frame could not be used during message exchanging.
// This type can be used if you want low level access to websocket.
type Frame struct {
	max    uint64
	op     []byte
	mask   []byte
	status []byte
	b      []byte
}

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
			max:    maxPayloadSize,
			op:     make([]byte, opSize),
			mask:   make([]byte, maskSize),
			status: make([]byte, statusSize),
			b:      make([]byte, 0, 128),
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

// Reset resets all Frame values to be reused.
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
	case CodeText:
		mode = ModeText
	case CodeBinary:
		mode = ModeBinary
	default:
		mode = ModeNone
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

// Len returns b length based on Frame field of length bytes.
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

// MaskKey returns mask key if exist.
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

// Payload returns Frame b.
func (fr *Frame) Payload() []byte {
	return fr.b
}

// PayloadSize returns max b size.
func (fr *Frame) PayloadSize() uint64 {
	return fr.max
}

// SetPayloadSize sets max b size.
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

// SetMask sets mask key to mask the frame and enabled mask bit.
func (fr *Frame) SetMask(b []byte) {
	fr.op[1] |= maskBit
	copy(fr.mask, b[:4])
}

// UnsetMask drops mask bit.
func (fr *Frame) UnsetMask() {
	fr.op[1] ^= maskBit
}

// Write writes b to the frame's payload.
func (fr *Frame) Write(b []byte) (int, error) {
	n := len(b)
	fr.b = append(fr.b, b...)
	return n, nil
}

// SetPayload sets b as the frame's payload.
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

// Mask masks Frame b.
func (fr *Frame) Mask() {
	if len(fr.b) > 0 {
		fr.op[1] |= maskBit
		readMask(fr.mask)
		mask(fr.mask, fr.b)
	}
}

// Unmask unmasks Frame b.
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

// WriteTo marshals the frame and writes the frame into wr.
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

// Status returns StatusCode from request b.
func (fr *Frame) Status() (status StatusCode) {
	status = StatusCode(
		binary.BigEndian.Uint16(fr.status),
	)
	return
}

// SetStatus sets status code to the request.
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
	errReadingLen    = errors.New("error reading b length")
	errReadingMask   = errors.New("error reading mask")
	errLenTooBig     = errors.New("message length is bigger than expected")
	errStatusLen     = errors.New("length of the status must be = 2")
)

const limitLen = 1 << 32

/*
func (fr *Frame) readFrom(br io.Reader) (int64, error) {
	var err error
	var n, m, i int

	for i = 0; i < 2; i += n {
		n, err = br.Read(fr.op[i:2])
		if err != nil {
			break
		}
	}
	if i < 2 {
		err = errReadingHeader
	}

	if err == nil {
		m = fr.mustRead() + 2
		if m > 2 { // reading length
			for i = 2; i < m; i += n {
				n, err = br.Read(fr.op[i:m])
				if err != nil {
					break
				}
			}
			if i < m {
				err = errReadingLen
			}
		}

		if err == nil && fr.IsMasked() { // reading mask
			for i = 0; i < 4; i += n {
				n, err = br.Read(fr.mask[i:4])
				if err != nil {
					break
				}
			}
			if i < 4 {
				err = errReadingMask
			}
		}

		if err == nil { // reading b
			fr.op[2] &= 127 // hot path to prevent overflow
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
						for i = 0; i < 2; i += n {
							n, err = br.Read(fr.status[i:2])
							if err != nil {
								break
							}
						}
						if i < 2 {
							err = errStatusLen
						}
					}

					if err == nil {
						fr.b = fr.b[:nn]
						for i := uint64(0); i < nn; i += uint64(n) {
							n, err = br.Read(fr.b[i:nn])
							if err != nil {
								break
							}
						}
					}
				}
			}
		}
	}
	return int64(n), err
}
*/

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
