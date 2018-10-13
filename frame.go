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
	b := make([]byte, 14)
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
	max     uint64
	status  []byte
	raw     []byte
	rawCopy []byte
	mask    []byte
	payload []byte
}

var framePool = sync.Pool{
	New: func() interface{} {
		fr := &Frame{
			max:     maxPayloadSize,
			mask:    make([]byte, 4),
			status:  make([]byte, 2),
			raw:     make([]byte, maxHeaderSize),
			rawCopy: make([]byte, maxHeaderSize),
			payload: make([]byte, maxPayloadSize),
		}
		fr.payload = fr.payload[:0]
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
	fr.payload = fr.payload[:0]
}

func (fr *Frame) resetHeader() {
	fr.mask = fr.mask[:4]
	fr.raw = fr.raw[:maxHeaderSize]
	fr.rawCopy = fr.rawCopy[:maxHeaderSize]
	copy(fr.raw, zeroBytes)
	copy(fr.mask, zeroBytes)
	copy(fr.status, zeroBytes)
	copy(fr.rawCopy, zeroBytes)
}

// Reset resets all Frame values to be reused.
func (fr *Frame) Reset() {
	fr.resetHeader()
	fr.resetPayload()
}

// IsFin checks if FIN bit is set.
func (fr *Frame) IsFin() bool {
	return fr.raw[0]&finBit != 0
}

// HasRSV1 checks if RSV1 bit is set.
func (fr *Frame) HasRSV1() bool {
	return fr.raw[0]&rsv1Bit != 0
}

// HasRSV2 checks if RSV2 bit is set.
func (fr *Frame) HasRSV2() bool {
	return fr.raw[0]&rsv2Bit != 0
}

// HasRSV3 checks if RSV3 bit is set.
func (fr *Frame) HasRSV3() bool {
	return fr.raw[0]&rsv3Bit != 0
}

// Code returns the code set in fr.
func (fr *Frame) Code() Code {
	return Code(fr.raw[0] & 15)
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
	return fr.raw[1]&maskBit != 0
}

// Len returns payload length based on Frame field of length bytes.
func (fr *Frame) Len() (length uint64) {
	length = uint64(fr.raw[1] & 127)
	switch length {
	case 126:
		if len(fr.raw) < 4 {
			length = 0
		} else {
			length = uint64(binary.BigEndian.Uint16(fr.raw[2:]))
		}
	case 127:
		if len(fr.raw) < 10 {
			length = 0
		} else {
			length = binary.BigEndian.Uint64(fr.raw[2:])
		}
	}
	return length
}

// MaskKey returns mask key if exist.
func (fr *Frame) MaskKey() []byte {
	return fr.mask
}

func (fr *Frame) parseStatus() {
	copy(fr.status[:2], fr.payload[:2])
	fr.payload = fr.payload[2:]
}

// Payload returns Frame payload.
func (fr *Frame) Payload() []byte {
	if fr.IsClose() && !fr.hasStatus() {
		fr.parseStatus()
	}
	return fr.payload
}

// PayloadSize returns max payload size.
func (fr *Frame) PayloadSize() uint64 {
	return fr.max
}

// SetPayloadSize sets max payload size.
func (fr *Frame) SetPayloadSize(size uint64) {
	fr.max = size
}

// SetFin sets FIN bit.
func (fr *Frame) SetFin() {
	fr.raw[0] |= finBit
}

// SetRSV1 sets RSV1 bit.
func (fr *Frame) SetRSV1() {
	fr.raw[0] |= rsv1Bit
}

// SetRSV2 sets RSV2 bit.
func (fr *Frame) SetRSV2() {
	fr.raw[0] |= rsv2Bit
}

// SetRSV3 sets RSV3 bit.
func (fr *Frame) SetRSV3() {
	fr.raw[0] |= rsv3Bit
}

// SetCode sets code bits.
func (fr *Frame) SetCode(code Code) {
	// TODO: Check non-reserved fields.
	code &= 15
	fr.raw[0] &= 15 << 4
	fr.raw[0] |= uint8(code)
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
	fr.raw[1] |= maskBit
	fr.mask = append(fr.mask[:0], b...)
}

// UnsetMask drops mask bit.
func (fr *Frame) UnsetMask() {
	fr.raw[1] ^= maskBit
}

// Write writes b to the frame payload.
func (fr *Frame) Write(b []byte) (int, error) {
	n := len(fr.payload)
	fr.setPayload(n, b)
	return n, nil
}

// SetPayload sets payload to fr.
func (fr *Frame) SetPayload(b []byte) {
	fr.setPayload(0, b)
}

func (fr *Frame) setPayload(i int, b []byte) {
	// TODO: Check max size
	fr.payload = append(fr.payload[:i], b...)
}

func (fr *Frame) setLengthPayload() {
	n := len(fr.payload)
	if fr.hasStatus() {
		n += 2
	}
	switch {
	case n > 65535:
		fr.setLength(127)
		binary.BigEndian.PutUint64(fr.raw[2:], uint64(n))
	case n > 125:
		fr.setLength(126)
		binary.BigEndian.PutUint16(fr.raw[2:], uint16(n))
	default:
		fr.setLength(n)
	}
}

func (fr *Frame) setLength(n int) {
	fr.raw[1] |= uint8(n)
}

// Mask masks Frame payload.
func (fr *Frame) Mask() {
	if len(fr.payload) > 0 {
		fr.raw[1] |= maskBit
		readMask(fr.mask[:4])
		mask(fr.mask, fr.payload)
	}
}

// Unmask unmasks Frame payload.
func (fr *Frame) Unmask() {
	key := fr.MaskKey()
	if len(key) == 4 {
		mask(key, fr.payload)
		fr.UnsetMask()
	}
}

func (fr *Frame) hasStatus() bool {
	return fr.status[0] > 0 && fr.status[1] > 0
}

// WriteTo flushes Frame data into wr.
func (fr *Frame) WriteTo(wr io.Writer) (n uint64, err error) {
	var nn int

	err = fr.prepare()
	if err == nil {
		nn, err = wr.Write(fr.raw)
		if err == nil {
			n += uint64(nn)
			ln := fr.Len()
			// writing status
			if fr.hasStatus() {
				ln -= 2
				nn, err = wr.Write(fr.status[:2])
				n += uint64(nn)
			}
			// writing payload
			if err == nil && ln > 0 {
				nn, err = wr.Write(fr.payload[:ln])
				n += uint64(nn)
			}
		}
	}
	return
}

func (fr *Frame) prepare() (err error) {
	fr.setLengthPayload()
	fr.rawCopy = append(fr.rawCopy[:0], fr.raw...)
	fr.raw = append(fr.raw[:0], fr.rawCopy[:2]...)

	err = fr.appendByLen()
	if err != nil {
		fr.raw = fr.raw[:maxHeaderSize]
	} else if fr.IsMasked() {
		fr.raw = append(fr.raw, fr.mask[:4]...)
	}
	return
}

// Status returns StatusCode from request payload.
func (fr *Frame) Status() (status StatusCode) {
	if fr.IsClose() && !fr.hasStatus() {
		fr.parseStatus()
	}
	status = StatusCode(
		binary.BigEndian.Uint16(fr.status[:2]),
	)
	return
}

// SetStatus sets status code to the request.
//
// Status code is usually used in Close request.
func (fr *Frame) SetStatus(status StatusCode) {
	binary.BigEndian.PutUint16(fr.status[:2], uint16(status))
}

func (fr *Frame) mustRead() (n int) {
	n = int(fr.raw[1] & 127)
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

func (fr *Frame) appendByLen() (err error) {
	n := fr.mustRead()
	switch n {
	case 8:
		if len(fr.rawCopy) < 10 {
			err = errBadHeaderSize
		} else {
			fr.raw = append(fr.raw, fr.rawCopy[2:10]...)
		}
	case 2:
		if len(fr.rawCopy) < 4 {
			err = errBadHeaderSize
		} else {
			fr.raw = append(fr.raw, fr.rawCopy[2:4]...)
		}
	}
	return
}

var (
	EOF                = io.EOF
	errMalformedHeader = errors.New("Malformed header")
	errBadHeaderSize   = errors.New("Header size is insufficient")
)

// ReadFrom fills fr reading from rd.
func (fr *Frame) ReadFrom(rd io.Reader) (nn uint64, err error) {
	return fr.readFrom(rd)
}

var (
	errReadingHeader = errors.New("error reading frame header")
	errReadingLen    = errors.New("error reading payload length")
	errReadingMask   = errors.New("error reading mask")
)

func (fr *Frame) readFrom(br io.Reader) (nn uint64, err error) {
	var n, m int

	fr.raw = fr.raw[:maxHeaderSize]
	n, err = br.Read(fr.raw[:2])
	if err == nil && n < 2 {
		err = errReadingHeader
	}
	if err == nil {
		m = fr.mustRead() + 2
		if m > 2 { // reading length
			n, err = br.Read(fr.raw[2:m])
			if n+2 < m {
				err = errReadingLen
			} else {
				n = m
			}
		}
		if err == nil && fr.IsMasked() { // reading mask
			m, err = br.Read(fr.raw[n : n+4])
			if m < 4 {
				err = errReadingMask
			} else {
				copy(fr.mask[:4], fr.raw[n:n+4])
			}
		}
		if err == nil { // reading payload
			if fr.Len() > fr.max {
				err = fmt.Errorf("Max payload size exceeded (%d < %d)", fr.max, fr.Len())
			} else {
				nn = fr.Len()
				if nn < 0 {
					err = errNegativeLen
				} else if nn > 0 {
					fr.payload = fr.payload[:nn]
					n, err = br.Read(fr.payload)
					nn = uint64(n)
				}
			}
		}
	}
	return
}

var errNegativeLen = errors.New("Negative len")
