package fastws

import (
	"context"
	"io"
	"net"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/valyala/fasthttp"
	nyws "nhooyr.io/websocket"
)

type fakeServerConn struct {
	net.TCPConn
	ln            *fakeListener
	requestsCount int
	pos           int
	closed        uint32
}

func (c *fakeServerConn) Read(b []byte) (int, error) {
	nn := 0
	reqLen := len(c.ln.request)
	for len(b) > 0 {
		if c.requestsCount == 0 {
			if nn == 0 {
				return 0, io.EOF
			}
			return nn, nil
		}
		pos := c.pos % reqLen
		n := copy(b, c.ln.request[pos:])
		b = b[n:]
		nn += n
		c.pos += n
		if n+pos == reqLen {
			c.requestsCount--
		}
	}
	return nn, nil
}

func (c *fakeServerConn) Write(b []byte) (int, error) {
	return len(b), nil
}

var fakeAddr = net.TCPAddr{
	IP:   []byte{1, 2, 3, 4},
	Port: 12345,
}

func (c *fakeServerConn) RemoteAddr() net.Addr {
	return &fakeAddr
}

func (c *fakeServerConn) Close() error {
	if atomic.AddUint32(&c.closed, 1) == 1 {
		c.ln.ch <- c
	}
	return nil
}

func (c *fakeServerConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *fakeServerConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type fakeListener struct {
	lock            sync.Mutex
	requestsCount   int
	requestsPerConn int
	request         []byte
	ch              chan *fakeServerConn
	done            chan struct{}
	closed          bool
}

func (ln *fakeListener) Accept() (net.Conn, error) {
	ln.lock.Lock()
	if ln.requestsCount == 0 {
		ln.lock.Unlock()
		for len(ln.ch) < cap(ln.ch) {
			time.Sleep(10 * time.Millisecond)
		}
		ln.lock.Lock()
		if !ln.closed {
			close(ln.done)
			ln.closed = true
		}
		ln.lock.Unlock()
		return nil, io.EOF
	}
	requestsCount := ln.requestsPerConn
	if requestsCount > ln.requestsCount {
		requestsCount = ln.requestsCount
	}
	ln.requestsCount -= requestsCount
	ln.lock.Unlock()

	c := <-ln.ch
	c.requestsCount = requestsCount
	c.closed = 0
	c.pos = 0

	return c, nil
}

func (ln *fakeListener) Close() error {
	return nil
}

func (ln *fakeListener) Addr() net.Addr {
	return &fakeAddr
}

func newFakeListener(requestsCount, clientsCount, requestsPerConn int, request []byte) *fakeListener {
	ln := &fakeListener{
		requestsCount:   requestsCount,
		requestsPerConn: requestsPerConn,
		request:         request,
		ch:              make(chan *fakeServerConn, clientsCount),
		done:            make(chan struct{}),
	}
	for i := 0; i < clientsCount; i++ {
		ln.ch <- &fakeServerConn{
			ln: ln,
		}
	}
	return ln
}

func buildUpgrade() (s []byte) {
	key := makeRandKey(nil)
	s = append(s, "GET / HTPP/1.1\r\n"+
		"Host: localhost:8080\r\n"+
		"Connection: Upgrade\r\n"+
		"Upgrade: websocket\r\n"+
		"Sec-WebSocket-Version: 13\r\n"+
		"Sec-WebSocket-Key: "+string(key)+"\r\n\r\n"...)
	s = append(s, []byte{129, 37, 109, 97, 107, 101, 32, 102, 97, 115, 116, 104, 116, 116, 112, 32, 103, 114, 101, 97, 116, 32, 97, 103, 97, 105, 110, 32, 119, 105, 116, 104, 32, 72, 84, 84, 80, 47, 50}...)
	return s
}

func benchmarkFastServer(b *testing.B, clients, count int) {
	s := fasthttp.Server{
		Handler: Upgrade(func(conn *Conn) {
			var err error
			var bf []byte
			for {
				_, bf, err = conn.ReadMessage(bf)
				if err != nil {
					if err == io.EOF {
						break
					}
					panic(err)
				}
			}
			conn.Close()
		}),
	}
	ch := make(chan struct{}, 1)
	ln := newFakeListener(b.N, clients, count, buildUpgrade())

	go func() {
		s.Serve(ln)
		ch <- struct{}{}
	}()

	<-ln.done

	select {
	case <-ch:
	case <-time.After(time.Second * 10):
		b.Fatal("timeout")
	}
}

func benchmarkGorillaServer(b *testing.B, clients, count int) {
	s := http.Server{
		Handler: &handler{b},
	}
	ch := make(chan struct{}, 1)
	ln := newFakeListener(b.N, clients, count, buildUpgrade())

	go func() {
		s.Serve(ln)
		ch <- struct{}{}
	}()

	<-ln.done
	select {
	case <-ch:
	case <-time.After(time.Second * 10):
		b.Fatal("timeout")
	}
}

var upgrader = websocket.Upgrader{}

type handler struct {
	b *testing.B
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.b.Fatal(err)
		return
	}
	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
	}
	c.Close()
}

type handlerNhooyr struct {
	b *testing.B
}

func (h *handlerNhooyr) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := nyws.Accept(w, r, &nyws.AcceptOptions{
		CompressionMode: nyws.CompressionDisabled, // disable compression to be fair
	})
	if err != nil {
		h.b.Fatal(err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	for {
		_, _, err := c.Read(ctx)
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
	}
	c.Close(nyws.StatusNormalClosure, "")
}

func benchmarkNhooyrServer(b *testing.B, clients, count int) {
	s := http.Server{
		Handler: &handlerNhooyr{b},
	}
	ch := make(chan struct{}, 1)
	ln := newFakeListener(b.N, clients, count, buildUpgrade())

	go func() {
		s.Serve(ln)
		ch <- struct{}{}
	}()

	<-ln.done
	select {
	case <-ch:
	case <-time.After(time.Second * 10):
		b.Fatal("timeout")
	}
}

func Benchmark1000FastClientsPer10Messages(b *testing.B) {
	benchmarkFastServer(b, 1000, 10)
}

func Benchmark1000FastClientsPer100Messages(b *testing.B) {
	benchmarkFastServer(b, 1000, 100)
}

func Benchmark1000FastClientsPer1000Messages(b *testing.B) {
	benchmarkFastServer(b, 1000, 1000)
}

func Benchmark1000GorillaClientsPer10Messages(b *testing.B) {
	benchmarkGorillaServer(b, 1000, 10)
}

func Benchmark1000GorillaClientsPer100Messages(b *testing.B) {
	benchmarkGorillaServer(b, 1000, 100)
}

func Benchmark1000GorillaClientsPer1000Messages(b *testing.B) {
	benchmarkGorillaServer(b, 1000, 1000)
}

func Benchmark1000NhooyrClientsPer10Messages(b *testing.B) {
	benchmarkNhooyrServer(b, 1000, 10)
}

func Benchmark1000NhooyrClientsPer100Messages(b *testing.B) {
	benchmarkNhooyrServer(b, 1000, 100)
}

func Benchmark1000NhooyrClientsPer1000Messages(b *testing.B) {
	benchmarkNhooyrServer(b, 1000, 1000)
}

func Benchmark100FastMsgsPerConn(b *testing.B) {
	benchmarkFastServer(b, runtime.NumCPU(), 100)
}

func Benchmark1000FastMsgsPerConn(b *testing.B) {
	benchmarkFastServer(b, runtime.NumCPU(), 1000)
}

func Benchmark10000FastMsgsPerConn(b *testing.B) {
	benchmarkFastServer(b, runtime.NumCPU(), 10000)
}

func Benchmark100000FastMsgsPerConn(b *testing.B) {
	benchmarkFastServer(b, runtime.NumCPU(), 100000)
}

func Benchmark100GorillaMsgsPerConn(b *testing.B) {
	benchmarkGorillaServer(b, runtime.NumCPU(), 100)
}

func Benchmark1000GorillaMsgsPerConn(b *testing.B) {
	benchmarkGorillaServer(b, runtime.NumCPU(), 1000)
}

func Benchmark10000GorillaMsgsPerConn(b *testing.B) {
	benchmarkGorillaServer(b, runtime.NumCPU(), 10000)
}

func Benchmark100000GorillaMsgsPerConn(b *testing.B) {
	benchmarkGorillaServer(b, runtime.NumCPU(), 100000)
}

func Benchmark100NhooyrMsgsPerConn(b *testing.B) {
	benchmarkNhooyrServer(b, runtime.NumCPU(), 100)
}

func Benchmark1000NhooyrMsgsPerConn(b *testing.B) {
	benchmarkNhooyrServer(b, runtime.NumCPU(), 1000)
}

func Benchmark10000NhooyrMsgsPerConn(b *testing.B) {
	benchmarkNhooyrServer(b, runtime.NumCPU(), 10000)
}

func Benchmark100000NhooyrMsgsPerConn(b *testing.B) {
	benchmarkNhooyrServer(b, runtime.NumCPU(), 100000)
}
