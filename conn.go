package fastws

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

type StatusCode uint16

// TODO: doc for status

const (
	StatusUndefined         StatusCode = 0
	StatusNone                         = 1000
	StatusGoAway                       = 1001
	StatusProtocolError                = 1002
	StatusNotAcceptable                = 1003
	StatusReserved                     = 1004
	StatusNotConsistent                = 1007
	StatusViolation                    = 1008
	StatusTooBig                       = 1009
	StatuseExtensionsNeeded            = 1010
	StatusUnexpected                   = 1011
)

// Mode is the mode in which the bytes are sended.
//
// https://tools.ietf.org/html/rfc6455#section-5.6
type Mode uint8

const (
	ModeNone Mode = iota
	ModeText
	ModeBinary
)

var (
	connPool sync.Pool
)

var (
	zeroTime        = time.Time{}
	defaultDeadline = time.Second * 8
)

// Conn represents websocket connection handler.
//
// This handler is compatible with io.Reader, io.ReaderFrom, io.Writer, io.WriterTo
type Conn struct {
	c net.Conn

	server   bool
	compress bool

	// mutex for reading and writing
	lbr, lbw sync.Mutex

	// Mode indicates Write default mode.
	Mode Mode

	// ReadTimeout ...
	ReadTimeout time.Duration

	// WriteTimeout ...
	WriteTimeout time.Duration

	// MaxPayloadSize prevents huge memory allocation.
	//
	// By default MaxPayloadSize is 4096.
	MaxPayloadSize uint64
}

// LocalAddr returns local address.
func (conn *Conn) LocalAddr() net.Addr {
	return conn.c.LocalAddr()
}

// RemoteAddr returns peer remote address.
func (conn *Conn) RemoteAddr() net.Addr {
	return conn.c.RemoteAddr()
}

// SetDeadline calls net.Conn.SetDeadline
func (conn *Conn) SetDeadline(t time.Time) error {
	return conn.c.SetDeadline(t)
}

// SetReadDeadline calls net.Conn.SetReadDeadline
func (conn *Conn) SetReadDeadline(t time.Time) error {
	return conn.c.SetReadDeadline(t)
}

// SetWriteDeadline calls net.Conn.SetWriteDeadline
func (conn *Conn) SetWriteDeadline(t time.Time) error {
	return conn.c.SetWriteDeadline(t)
}

func acquireConn(c net.Conn) (conn *Conn) {
	ci := connPool.Get()
	if ci != nil {
		conn = ci.(*Conn)
	} else {
		conn = &Conn{}
	}
	conn.Reset(c)
	return conn
}

func releaseConn(conn *Conn) {
	connPool.Put(conn)
}

// Reset resets conn values setting c as default connection endpoint.
func (conn *Conn) Reset(c net.Conn) {
	if conn.c != nil {
		conn.c.Close() // hard close
	}
	conn.ReadTimeout = defaultDeadline
	conn.WriteTimeout = defaultDeadline
	conn.MaxPayloadSize = maxPayloadSize
	conn.compress = false
	conn.server = false
	conn.c = c
}

// WriteFrame writes fr to the connection endpoint.
func (conn *Conn) WriteFrame(fr *Frame) (int, error) {
	conn.lbw.Lock()
	defer conn.lbw.Unlock()

	if conn.c == nil {
		return 0, EOF
	}
	// TODO: Compress

	fr.SetPayloadSize(conn.MaxPayloadSize)
	if conn.WriteTimeout > 0 {
		conn.c.SetWriteDeadline(time.Now().Add(conn.WriteTimeout))
	}

	nn, err := fr.WriteTo(conn.c)
	if err == nil {
		conn.c.SetWriteDeadline(zeroTime)
	}
	return int(nn), err
}

// ReadFrame fills fr with the next connection frame.
//
// This function responds automatically to PING and PONG messages.
func (conn *Conn) ReadFrame(fr *Frame) (nn int, err error) {
	conn.lbr.Lock()
	defer conn.lbr.Unlock()

	if conn.c == nil {
		err = EOF
	} else {
		var n int64
		if conn.ReadTimeout > 0 {
			conn.c.SetReadDeadline(time.Now().Add(conn.ReadTimeout))
		}

		fr.SetPayloadSize(conn.MaxPayloadSize)

		n, err = fr.ReadFrom(conn.c)
		if err == nil {
			nn = int(n)
			conn.c.SetReadDeadline(zeroTime)
		}
	}

	return
}

// WriteString writes b to conn using conn.Mode as default.
func (conn *Conn) WriteString(b string) (int, error) {
	return conn.Write(s2b(b))
}

// Write writes b using conn.Mode as default.
func (conn *Conn) Write(b []byte) (int, error) {
	return conn.write(conn.Mode, b)
}

// WriteMessage writes b to conn using mode.
func (conn *Conn) WriteMessage(mode Mode, b []byte) (int, error) {
	return conn.write(mode, b)
}

// ReadMessage reads next message from conn and returns the mode, b and/or error.
//
// b is used to avoid extra allocations and can be nil.
func (conn *Conn) ReadMessage(b []byte) (Mode, []byte, error) {
	return conn.read(b)
}

// SendCodeString writes code, status and message to conn as SendCode does.
func (conn *Conn) SendCodeString(code Code, status StatusCode, b string) error {
	return conn.SendCode(code, status, s2b(b))
}

// SendCode writes code, status and message to conn.
//
// status is used by CodeClose to report any close status (as HTTP responses). Can be 0.
// b can be nil.
func (conn *Conn) SendCode(code Code, status StatusCode, b []byte) error {
	fr := AcquireFrame()
	fr.SetFin()
	fr.SetCode(code)
	if status > 0 {
		fr.SetStatus(status)
	}
	if b != nil {
		fr.Write(b)
	}
	if !conn.server && !fr.IsMasked() {
		fr.Mask()
	}
	_, err := conn.WriteFrame(fr)
	ReleaseFrame(fr)

	return err
}

// NextFrame reads next connection frame and returns if there were no error.
//
// If NextFrame fr is not nil do not forget to ReleaseFrame(fr)
// This function responds automatically to PING and PONG messages.
func (conn *Conn) NextFrame() (fr *Frame, err error) {
	fr = AcquireFrame()
	_, err = conn.ReadFrame(fr)
	if err != nil {
		ReleaseFrame(fr)
		fr = nil
	}
	return fr, err
}

func (conn *Conn) checkRequirements(fr *Frame, betweenContinuation bool) (c bool, err error) {
	if !conn.server && fr.IsMasked() { // if server masked content
		err = fmt.Errorf("Server sent masked content")
		return
	}
	isFin := fr.IsFin()

	switch {
	case fr.IsPing():
		if !isFin && !betweenContinuation {
			err = errControlMustNotBeFragmented
		} else {
			err = conn.SendCode(CodePong, 0, fr.Payload())
			c = true
		}
	case fr.IsPong():
		if !isFin && !betweenContinuation {
			err = errControlMustNotBeFragmented
		} else {
			c = true
		}
	case fr.IsClose():
		if !isFin && !betweenContinuation {
			err = errControlMustNotBeFragmented
		} else {
			err = conn.ReplyClose(fr)
			if err == nil {
				err = EOF
			}
		}
		c = false
	}

	return
}

// TODO: Add timeout
func (conn *Conn) write(mode Mode, b []byte) (int, error) {
	if conn.c == nil {
		return 0, EOF
	}

	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	fr.SetFin()
	if mode == ModeBinary {
		fr.SetBinary()
	} else {
		fr.SetText()
	}

	fr.SetPayload(b)
	if !conn.server {
		fr.Mask()
	}
	return conn.WriteFrame(fr)
}

// TODO: Add timeout
func (conn *Conn) read(b []byte) (Mode, []byte, error) {
	if conn.c == nil {
		return 0, b, EOF
	}

	var err error
	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	b, err = conn.ReadFull(b, fr)

	return fr.Mode(), b, err
}

// ReadFull ...
func (conn *Conn) ReadFull(b []byte, fr *Frame) ([]byte, error) {
	var c bool
	var err error
	betweenContinue := false

	for {
		fr.Reset()

		_, err = conn.ReadFrame(fr)
		if err != nil {
			break
		}
		if fr.IsMasked() {
			fr.Unmask()
		}

		c, err = conn.checkRequirements(fr, betweenContinue)
		if err != nil {
			break
		}
		if c {
			continue
		}

		if betweenContinue && !fr.IsFin() && !fr.IsContinuation() && !fr.IsControl() {
			err = fmt.Errorf("%s. Got %d", errFrameBetweenContinuation, fr.Code())
			break
		}

		if p := fr.Payload(); len(p) > 0 {
			b = append(b, p...)
		}

		if fr.IsFin() { // unfragmented message
			break
		}

		// fragmented
		betweenContinue = true
	}
	if err != nil {
		var nErr error
		switch err {
		case errLenTooBig:
			nErr = conn.sendClose(StatusTooBig, nil)
		case errStatusLen:
			nErr = conn.sendClose(StatusNotConsistent, nil)
		case errControlMustNotBeFragmented, errFrameBetweenContinuation:
			nErr = conn.sendClose(StatusProtocolError, nil)
		}
		if nErr != nil {
			err = fmt.Errorf("error closing connection due to %s: %s", err, nErr)
		}
	}

	return b, err
}

var (
	errControlMustNotBeFragmented = errors.New("control frames must not be fragmented")
	errFrameBetweenContinuation   = errors.New("received frame between continuation frames")
)

func (conn *Conn) sendClose(status StatusCode, b []byte) (err error) {
	fr := AcquireFrame()
	fr.SetFin()
	fr.SetClose()
	// If there is a body, the first two bytes of
	// the body MUST be a 2-byte unsigned integer

	if status == StatusUndefined {
		status = StatusNone
	}
	fr.SetStatus(status)

	if len(b) > 0 {
		fr.SetPayload(b)
	}
	if !conn.server {
		fr.Mask()
	}
	_, err = fr.WriteTo(conn.c)
	ReleaseFrame(fr)
	return
}

var errNilFrame = errors.New("frame cannot be nil")

// ReplyClose is used to reply to CodeClose.
func (conn *Conn) ReplyClose(fr *Frame) (err error) {
	if fr == nil {
		return errNilFrame
	}
	fr.SetFin()
	fr.SetClose()
	_, err = conn.WriteFrame(fr)
	return
}

// Close closes the websocket connection.
func (conn *Conn) Close() error {
	return conn.CloseString("")
}

// CloseString sends b as close reason and closes the descriptor.
//
// When connection is handled by server the connection is closed automatically.
func (conn *Conn) CloseString(b string) error {
	conn.lbw.Lock()
	conn.lbr.Lock()
	defer conn.lbw.Unlock()
	defer conn.lbr.Unlock()

	if conn.c == nil {
		return EOF
	}

	var bb []byte
	var err error

	if b != "" {
		bb = s2b(b)
	}

	conn.c.SetWriteDeadline(time.Now().Add(conn.WriteTimeout))
	err = conn.sendClose(StatusNone, bb) // TODO: Edit status code
	if err != nil {
		conn.c.SetWriteDeadline(zeroTime)
	} else {
		fr := AcquireFrame()
		conn.c.SetReadDeadline(time.Now().Add(conn.ReadTimeout))

		_, err = fr.ReadFrom(conn.c)
		if err != nil {
			conn.c.SetReadDeadline(zeroTime)
		} else {
			err = conn.c.Close()
			if err == nil {
				conn.c = nil
			}
		}
		ReleaseFrame(fr)
	}
	return err
}
