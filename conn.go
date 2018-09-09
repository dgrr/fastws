package fastws

import (
	"bufio"
	"io"
	"net"
	"sync"
)

type StatusCode uint16

const (
	StatusNone              = 1000
	StatusGoAway            = 1001
	StatusProtocolError     = 1002
	StatusNotAcceptable     = 1003
	StatusReserved          = 1004
	StatusNotConsistent     = 1007
	StatusViolation         = 1008
	StatusTooBig            = 1009
	StatuseExtensionsNeeded = 1010
	StatusUnexpected        = 1011
)

// Mode is the mode in which the bytes are sended.
type Mode uint8

const (
	ModeNone Mode = iota
	ModeText
	ModeBinary
)

var connPool sync.Pool

// Conn represents websocket connection handler.
//
// This handler is compatible with io.Reader, io.ReaderFrom, io.Writer, io.WriterTo
type Conn struct {
	lck sync.Locker

	c net.Conn

	wpool sync.Pool
	rpool sync.Pool

	server bool

	// extra bytes
	extra []byte

	// Mode indicates Write and Read default mode.
	Mode Mode
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

func (conn *Conn) acquireWriter() (bw *bufio.Writer) {
	w := conn.wpool.Get()
	if w == nil {
		bw = bufio.NewWriter(conn.c)
	} else {
		bw = w.(*bufio.Writer)
		bw.Reset(conn.c)
	}
	return bw
}

func (conn *Conn) releaseReader(br *bufio.Reader) {
	conn.rpool.Put(br)
}

func (conn *Conn) releaseWriter(bw *bufio.Writer) {
	conn.wpool.Put(bw)
}

// Reset resets conn values setting c as default connection endpoint.
func (conn *Conn) Reset(c net.Conn) {
	if conn.c != nil {
		conn.Close(nil)
	}
	if conn.lck == nil {
		conn.lck = &sync.Mutex{}
	}
	conn.extra = conn.extra[:0]
	conn.c = c
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
	bw := conn.acquireWriter()
	nn, err := fr.WriteTo(bw)
	if err == nil {
		err = bw.Flush()
	}
	conn.releaseWriter(bw)
	return int(nn), err
}

// ReadFrame fills fr with the next connection frame.
func (conn *Conn) ReadFrame(fr *Frame) (nn int, err error) {
	br := conn.acquireReader()
	var n uint64
	for {
		n, err = fr.ReadFrom(br)
		nn = int(n)
		if err == nil {
			switch {
			case fr.IsPing():
				conn.SendCode(CodePong, 0, nil)
				fr.Reset()
				continue
			case fr.IsPong():
				fr.Reset()
				continue
			case fr.IsClose():
				err = EOF
			}
		}
		break
	}
	conn.releaseReader(br)
	return nn, err
}

// NextFrame reads next connection frame and returns if there were no error.
//
// If NextFrame fr is not nil do not forget to ReleaseFrame(fr)
func (conn *Conn) NextFrame() (fr *Frame, err error) {
	br := conn.acquireReader()
	fr = AcquireFrame()
	_, err = fr.ReadFrom(br)
	conn.releaseReader(br)
	if err != nil {
		ReleaseFrame(fr)
		fr = nil
	}
	return fr, err
}

// TODO: Add timeout
func (conn *Conn) write(mode Mode, b []byte) (int, error) {
	if conn.checkClose() {
		return 0, io.EOF
	}

	fr := AcquireFrame()
	defer ReleaseFrame(fr)

	// TODO: Apply continuation frames if b is large.
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
	var err error

	fr := AcquireFrame()
	//fr.setRsvFromConn(conn)
	//fr.setExtensionLength(conn.extensionLength)
	defer ReleaseFrame(fr)

	for !conn.checkClose() {
		_, err = conn.ReadFrame(fr)
		if err != nil {
			break
		}
		if conn.server && fr.IsMasked() {
			fr.Unmask()
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

// Close closes the connection sending CodeClose and b and closing the descriptor.
func (conn *Conn) Close(b []byte) error {
	if conn.checkClose() {
		return nil
	}

	err := conn.SendCode(CodeClose, StatusNone, b)
	if err == nil {
		err = conn.c.Close()
		if err == nil {
			conn.lck.Lock()
			conn.c = nil
			conn.lck.Unlock()
		}
	}
	return err
}

func (conn *Conn) checkClose() (closed bool) {
	conn.lck.Lock()
	closed = (conn.c == nil)
	conn.lck.Unlock()
	return
}

// TODO: https://tools.ietf.org/html/rfc6455#section-5.4
