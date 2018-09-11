package fastws

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

const (
	maxHeaderSize = 14
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
	extensionLen int

	rsv1 bool
	rsv2 bool
	rsv3 bool

	raw       []byte
	rawCopy   []byte
	mask      []byte
	payload   []byte
	extension []byte
}

var framePool = sync.Pool{
	New: func() interface{} {
		return &Frame{
			mask:    make([]byte, 4),
			raw:     make([]byte, maxHeaderSize),
			rawCopy: make([]byte, maxHeaderSize),
		}
	},
}

// AcquireFrame gets Frame from pool.
func AcquireFrame() *Frame {
	return framePool.Get().(*Frame)
}

// ReleaseFrame puts fr Frame into the pool.
func ReleaseFrame(fr *Frame) {
	fr.Reset()
	fr.hardReset()
	framePool.Put(fr)
}

func (fr *Frame) resetPayload() {
	fr.payload = fr.payload[:0]
	fr.extension = fr.extension[:0]
}

func (fr *Frame) resetHeader() {
	fr.extensionLen = 0
	fr.mask = fr.mask[:4]
	fr.raw = fr.raw[:maxHeaderSize]
	fr.rawCopy = fr.rawCopy[:maxHeaderSize]
	copy(fr.mask, zeroBytes)
	copy(fr.raw, zeroBytes)
	copy(fr.rawCopy, zeroBytes)
}

func (fr *Frame) hardReset() {
	fr.rsv1 = false
	fr.rsv2 = false
	fr.rsv3 = false
	fr.extensionLen = 0
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

// Payload returns Frame payload.
func (fr *Frame) Payload() []byte {
	return fr.payload
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
	n := len(b) + i
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
	fr.payload = append(fr.payload[:i], b...)
}

func (fr *Frame) setLength(n int) {
	fr.raw[1] |= uint8(n)
}

// SetExtensionLength sets the extension length.
func (fr *Frame) SetExtensionLength(n int) {
	// TODO: Support extensions
	fr.extensionLen = n
}

// Mask masks Frame payload.
func (fr *Frame) Mask() {
	fr.raw[1] |= maskBit
	readMask(fr.mask[:4])
	mask(fr.mask, fr.payload)
}

// Unmask unmasks Frame payload.
func (fr *Frame) Unmask() {
	key := fr.MaskKey()
	if len(key) == 4 {
		mask(key, fr.payload)
	}
}

// WriteTo flushes Frame data into wr.
func (fr *Frame) WriteTo(wr io.Writer) (n uint64, err error) {
	var nn int

	err = fr.prepare()
	if err == nil {
		nn, err = wr.Write(fr.raw)
		if err == nil && len(fr.payload) > 0 {
			n += uint64(nn)
			nn, err = wr.Write(fr.payload)
		}
		n += uint64(nn)
	}
	return
}

func (fr *Frame) prepare() (err error) {
	copy(fr.rawCopy, fr.raw)

	fr.raw = append(fr.raw[:0], fr.rawCopy[:2]...)
	err = fr.appendByLen()
	if err != nil {
		fr.raw = fr.raw[:maxHeaderSize]
	} else if fr.IsMasked() {
		fr.raw = append(fr.raw, fr.mask...)
	}
	return
}

func (fr *Frame) setError(status StatusCode) {
	if cap(fr.payload) < 2 {
		fr.payload = append(fr.payload, make([]byte, 2)...)
	}
	fr.payload = fr.payload[:2]
	binary.BigEndian.PutUint16(fr.payload, uint16(status))
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
	EOF                = errors.New("Closed connection")
	errMalformedHeader = errors.New("Malformed header")
	errBadHeaderSize   = errors.New("Header size is insufficient")
)

// ReadFrom fills fr reading from rd.
func (fr *Frame) ReadFrom(rd io.Reader) (nn uint64, err error) {
	switch r := rd.(type) {
	case *bufio.Reader:
		nn, err = fr.readBufio(r)
	default:
		nn, err = fr.readStd(rd)
	}
	return
}

func (fr *Frame) readBufio(br *bufio.Reader) (nn uint64, err error) {
	var n int
	var b []byte

	for {
		b, err = br.Peek(2)
		if err != nil {
			return
		}
		fr.raw = append(fr.raw[:0], b[0], b[1])
		br.Discard(2)

		n = fr.mustRead()
		if n > 0 { // reading length
			b, err = br.Peek(n)
			if err == nil {
				if len(b) < n {
					err = errReadingLen
				} else {
					fr.raw = append(fr.raw, b...)
					br.Discard(n)
				}
			}
		}
		if err == nil {
			if fr.IsMasked() { // reading mask
				b, err = br.Peek(4)
				if err == nil {
					if len(b) < 4 {
						err = errReadingMask
					} else {
						copy(fr.mask[:4], b)
						br.Discard(4)
					}
				}
			}
			if err == nil { // reading payload
				if fr.extensionLen > 0 {
					b, err = br.Peek(fr.extensionLen)
					if err != nil {
						break
					}
					fr.extension = append(fr.extension[:0], b...) // TODO: append to zero?
				}

				b, err = br.Peek(int(fr.Len()))
				if err == nil {
					if fr.IsMasked() {
						mask(fr.mask[:4], b)
						fr.UnsetMask()
					}
					fr.payload = append(fr.payload[:nn], b...)
					nn += uint64(len(b))
				}
			}
		}

		if !fr.IsContinuation() || err != nil {
			break
		}
		fr.resetHeader()
	}
	return
}

var (
	errReadingHeader    = errors.New("error reading frame header")
	errReadingLen       = errors.New("error reading payload length")
	errReadingMask      = errors.New("error reading mask")
	errReadingExtension = errors.New("error reading extension")
)

func (fr *Frame) readStd(br io.Reader) (nn uint64, err error) {
	var n, m int

	for {
		fr.raw = fr.raw[:maxHeaderSize]
		n, err = br.Read(fr.raw[:2])
		if err != nil {
			break
		}
		if n < 2 {
			err = errReadingHeader
			break
		}

		m = fr.mustRead()
		if m > 0 { // reading length
			n, err = br.Read(fr.raw[2 : m+2])
			if n < m {
				err = errReadingLen
			} else {
				m += 2
				n = m
			}
		}
		if fr.IsMasked() && err == nil { // reading mask
			m, err = br.Read(fr.raw[n : n+4])
			if m < 4 {
				err = errReadingMask
			} else {
				copy(fr.mask[:4], fr.raw[n:n+4])
			}
		}
		if err == nil { // reading payload
			if fr.extensionLen > 0 {
				fr.extension = fr.extension[:cap(fr.extension)]
				if n = len(fr.extension) - fr.extensionLen; n > 0 {
					fr.extension = append(fr.extension, make([]byte, n)...)
				}
				n, err = br.Read(fr.extension[:fr.extensionLen])
				if err != nil {
					break
				}
				if fr.extensionLen < n {
					err = errReadingExtension
				} else {
					fr.extension = fr.extension[:n]
				}
			}

			fr.payload = fr.payload[:cap(fr.payload)]
			if n = int(fr.Len()) - (len(fr.payload) - int(nn)); n > 0 {
				fr.payload = append(fr.payload, make([]byte, n)...)
			}
			n, err = br.Read(fr.payload[nn:])
			if err == nil {
				if fr.IsMasked() { // TODO: Needed?
					mask(fr.mask, fr.payload[nn:nn+uint64(n)])
					fr.UnsetMask()
				}
				nn += uint64(n)
				fr.payload = fr.payload[:nn]
			}
		}
		if !fr.IsContinuation() || err != nil {
			break
		}
		fr.resetHeader()
	}
	return
}
