package fastws

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type StatusCode uint16

// TODO: doc for status

const (
	StatusNone              StatusCode = 1000
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

// Conn represents websocket connection handler.
//
// This handler is compatible with io.Reader, io.ReaderFrom, io.Writer, io.WriterTo
type Conn struct {
	n int64
	c net.Conn

	wpool sync.Pool
	rpool sync.Pool

	server   bool
	compress bool
	closed   bool

	// extra bytes
	extra []byte

	// Mode indicates Write default mode.
	Mode Mode

	// MaxPayloadSize prevents huge memory allocation.
	//
	// By default MaxPayloadSize is 4096.
	MaxPayloadSize uint64
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

func (conn *Conn) acquireReader() (br *bufio.Reader) {
	r := conn.rpool.Get()
	if r == nil {
		br = bufio.NewReader(conn.c)
	} else {
		br = r.(*bufio.Reader)
		br.Reset(conn.c)
	}
	return br
}

func (conn *Conn) acquireWriter() (bw *bufferWriter) {
	w := conn.wpool.Get()
	if w == nil {
		bw = &bufferWriter{
			wr: conn.c,
		}
	} else {
		bw = w.(*bufferWriter)
		bw.Reset(conn.c)
	}
	return bw
}

func (conn *Conn) releaseReader(br *bufio.Reader) {
	conn.rpool.Put(br)
}

func (conn *Conn) releaseWriter(bw *bufferWriter) {
	conn.wpool.Put(bw)
}

// Reset resets conn values setting c as default connection endpoint.
func (conn *Conn) Reset(c net.Conn) {
	if conn.c != nil {
		conn.Close("")
	}
	conn.closed = false
	conn.n = 0
	conn.MaxPayloadSize = maxPayloadSize
	conn.extra = conn.extra[:0]
	conn.c = c
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
	return conn.read(b[:0])
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
		fr.setError(status)
	}
	if b != nil {
		fr.Write(b)
	}
	_, err := conn.WriteFrame(fr)
	ReleaseFrame(fr)
	return err
}

// WriteFrame writes fr to the connection endpoint.
func (conn *Conn) WriteFrame(fr *Frame) (int, error) {
	if conn.closed {
		return 0, io.EOF
	}
	atomic.AddInt64(&conn.n, 1)

	// TODO: Compress

	bw := conn.acquireWriter()
	nn, err := fr.WriteTo(bw)
	if err == nil {
		err = bw.Flush()
	}
	conn.releaseWriter(bw)
	atomic.AddInt64(&conn.n, -1)
	return int(nn), err
}

// ReadFrame fills fr with the next connection frame.
//
// This function responds automatically to PING and PONG messages.
func (conn *Conn) ReadFrame(fr *Frame) (nn int, err error) {
	if conn.closed {
		err = io.EOF
	} else {
		atomic.AddInt64(&conn.n, 1)
		br := conn.acquireReader()
		var n uint64

		if conn.closed {
			err = io.EOF
		} else {
			n, err = fr.ReadFrom(br)
			nn = int(n)
		}
		conn.releaseReader(br)
		atomic.AddInt64(&conn.n, -1)
	}
	return
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

func (conn *Conn) checkRequirements(fr *Frame) (c bool, err error) {
	if !conn.server && fr.IsMasked() { // if server masked content
		err = fmt.Errorf("Server sent masked content")
		return
	}

	switch {
	case fr.IsPing():
		conn.SendCode(CodePong, 0, nil)
		fr.Reset()
		c = true
	case fr.IsPong():
		fr.Reset()
		c = true
	case fr.IsClose():
		if fr.Len() > 0 {
			err = fmt.Errorf("%s: %s", EOF, fr.Payload())
		} else {
			err = EOF
		}
	}
	return
}

// TODO: Add timeout
func (conn *Conn) write(mode Mode, b []byte) (int, error) {
	if conn.closed {
		return 0, io.EOF
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
	if conn.closed {
		return 0, b, io.EOF
	}

	var c bool
	var err error
	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	for {
		_, err = conn.ReadFrame(fr)
		if err != nil {
			break
		}
		if err == nil {
			if c, err = conn.checkRequirements(fr); c {
				continue
			}
		}

		if conn.server && fr.IsMasked() {
			fr.Unmask()
			fr.UnsetMask()
		}

		b = append(b, fr.payload...)

		if fr.IsFin() {
			break
		}

		fr.resetHeader()
		fr.resetPayload()
	}
	return fr.Mode(), b, err
}

// Close sends b as close reason and closes the descriptor.
//
// When connection is handled by server the connection is closed automatically.
func (conn *Conn) Close(b string) error {
	conn.closed = true
	for atomic.LoadInt64(&conn.n) > 0 {
		time.Sleep(time.Millisecond * 20)
	}

	err := conn.SendCodeString(CodeClose, StatusNone, b)
	if err == nil {
		err = conn.c.Close()
		if err == nil {
			conn.c = nil
		}
	}
	return err
}
