package fastws

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// StatusCode is sent when closing a connection.
//
// The following constants have been defined by the RFC.
type StatusCode uint16

const (
	// StatusNone is used to let the peer know nothing happened.
	StatusNone StatusCode = 1000
	// StatusGoAway peer's error.
	StatusGoAway = 1001
	// StatusProtocolError problem with the peer's way to communicate.
	StatusProtocolError = 1002
	// StatusNotAcceptable when a request is not acceptable
	StatusNotAcceptable = 1003
	// StatusReserved when a reserved field have been used
	StatusReserved = 1004
	// StatusNotConsistent IDK
	StatusNotConsistent = 1007
	// StatusViolation a violation of the protocol happened
	StatusViolation = 1008
	// StatusTooBig payload bigger than expected
	StatusTooBig = 1009
	// StatuseExtensionsNeeded IDK
	StatuseExtensionsNeeded = 1010
	// StatusUnexpected IDK
	StatusUnexpected = 1011
)

// Mode is the mode in which the bytes are sended.
//
// https://tools.ietf.org/html/rfc6455#section-5.6
type Mode uint8

const (
	// ModeText defines to use a text mode
	ModeText Mode = iota
	// ModeBinary defines to use a binary mode
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
	c      net.Conn
	bf     *bufio.ReadWriter
	closed bool
	wg     sync.WaitGroup

	framer chan *Frame
	errch  chan error

	server   bool
	compress bool

	lck sync.Mutex

	userValues map[string]interface{}

	// Mode indicates Write default mode.
	Mode Mode

	// ReadTimeout ...
	ReadTimeout time.Duration

	// WriteTimeout ...
	WriteTimeout time.Duration

	// MaxPayloadSize prevents huge memory allocation.
	//
	// By default MaxPayloadSize is DefaultPayloadSize.
	MaxPayloadSize uint64
}

// UserValue returns the key associated value.
func (conn *Conn) UserValue(key string) interface{} {
	return conn.userValues[key]
}

// SetUserValue assigns a key to the given value
func (conn *Conn) SetUserValue(key string, value interface{}) {
	conn.userValues[key] = value
}

// LocalAddr returns local address.
func (conn *Conn) LocalAddr() net.Addr {
	return conn.c.LocalAddr()
}

// RemoteAddr returns peer remote address.
func (conn *Conn) RemoteAddr() net.Addr {
	return conn.c.RemoteAddr()
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

// DefaultPayloadSize defines the default payload size (when none was defined).
const DefaultPayloadSize = 1 << 20

// Reset resets conn values setting c as default connection endpoint.
func (conn *Conn) Reset(c net.Conn) {
	conn.framer = make(chan *Frame, 128)
	conn.errch = make(chan error, 128)
	conn.ReadTimeout = defaultDeadline
	conn.WriteTimeout = defaultDeadline
	conn.MaxPayloadSize = DefaultPayloadSize
	conn.compress = false
	conn.server = false
	conn.userValues = make(map[string]interface{})
	conn.c = c
	{
		cr := c.(io.Reader)
		br, ok := cr.(*bufio.Reader)
		if !ok {
			br = bufio.NewReader(c)
		}
		conn.bf = bufio.NewReadWriter(br, bufio.NewWriter(c))
	}
	conn.closed = false
	conn.wg.Add(1)
	go conn.readLoop()
}

func (conn *Conn) readLoop() {
	defer conn.wg.Done()
	defer close(conn.framer)

	for {
		fr := AcquireFrame()
		fr.SetPayloadSize(conn.MaxPayloadSize)

		_, err := fr.ReadFrom(conn.bf)
		if err != nil {
			if err != EOF && !strings.Contains(err.Error(), "closed") {
				var (
					ok   = true // it can only be false
					errn error
				)

				select {
				case errn, ok = <-conn.errch:
				default:
				}
				if ok {
					if errn != nil {
						conn.errch <- errn
					}
					conn.errch <- err
				}
			}
			ReleaseFrame(fr)
			return
		}
		conn.framer <- fr
	}
}

// WriteFrame writes fr to the connection endpoint.
func (conn *Conn) WriteFrame(fr *Frame) (int, error) {
	conn.lck.Lock()
	if conn.closed {
		conn.lck.Unlock()
		return 0, EOF
	}
	// TODO: Compress

	fr.SetPayloadSize(conn.MaxPayloadSize)

	if conn.WriteTimeout > 0 {
		conn.c.SetWriteDeadline(time.Now().Add(conn.WriteTimeout))
	}

	nn, err := fr.WriteTo(conn.bf)
	if err == nil {
		err = conn.bf.Flush()
	}
	conn.c.SetWriteDeadline(zeroTime)
	conn.lck.Unlock()

	return int(nn), err
}

// ReadFrame fills fr with the next connection frame.
func (conn *Conn) ReadFrame(fr *Frame) (nn int, err error) {
	var expire <-chan time.Time
	if conn.ReadTimeout > 0 {
		timer := time.NewTimer(conn.ReadTimeout)
		expire = timer.C
		defer timer.Stop()
	}

	var ok bool
	select {
	case fr2, ok := <-conn.framer:
		if !ok {
			err = EOF
		} else {
			fr2.CopyTo(fr)
			nn = fr.PayloadLen()
			ReleaseFrame(fr2)
		}
	case err, ok = <-conn.errch:
		if !ok {
			err = EOF
		}
	case <-expire:
		err = errors.New("i/o timeout")
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
//
// This function responds automatically to PING and PONG messages.
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

func (conn *Conn) write(mode Mode, b []byte) (int, error) {
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

func (conn *Conn) read(b []byte) (Mode, []byte, error) {
	var err error
	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	b, err = conn.ReadFull(b, fr)

	return fr.Mode(), b, err
}

// ReadFull will read the parsed frame fully and writing the payload into b.
//
// This function responds automatically to PING and PONG messages.
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
		conn.mustClose(err == nil)
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

	fr.SetStatus(status)

	if len(b) > 0 {
		fr.SetPayload(b)
	}
	if !conn.server {
		fr.Mask()
	}

	if conn.WriteTimeout == 0 {
		conn.lck.Lock()
		conn.c.SetWriteDeadline(time.Now().Add(time.Second * 5))
		conn.lck.Unlock()
	}
	_, err = conn.WriteFrame(fr)
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

	conn.WriteFrame(fr)

	return conn.mustClose(false)
}

// Close closes the websocket connection.
func (conn *Conn) Close() error {
	return conn.CloseString("")
}

// CloseString sends b as close reason and closes the descriptor.
//
// When connection is handled by server the connection is closed automatically.
func (conn *Conn) CloseString(b string) error {
	conn.lck.Lock()
	if conn.closed {
		conn.lck.Unlock()
		return EOF
	}
	conn.lck.Unlock()

	var bb []byte
	if b != "" {
		bb = s2b(b)
	}
	conn.sendClose(StatusNone, bb)

	return conn.mustClose(true)
}

func (conn *Conn) mustClose(wait bool) error {
	conn.lck.Lock()
	if conn.closed {
		conn.lck.Unlock()
		return EOF
	}
	conn.closed = true
	conn.lck.Unlock()

	conn.bf.Flush()
	close(conn.errch)

	if wait {
		var fr *Frame
		expire := time.After(time.Second * 5)
	loop:
		for {
			var ok bool
			select {
			case fr, ok = <-conn.framer:
				if !ok || fr.IsClose() { // read until the close frame
					break loop
				}
				ReleaseFrame(fr)
			case <-expire:
				break loop
			}
		}
		if fr != nil {
			ReleaseFrame(fr)
		}
	}

	err := conn.c.Close()
	conn.wg.Wait() // should return immediately after closing

	return err
}
