package fastws

import (
	"io"
	"net/http"
	"net/url"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func benchmarkFastServer(b *testing.B, clients int, count uint64) {
	var n uint64 = 0
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
					b.Fatal(err)
				}
			}
			conn.Close("")
		}),
	}
	ch := make(chan struct{}, 1)

	ln := fasthttputil.NewInmemoryListener()

	go func() {
		c := count * uint64(clients)
		for atomic.LoadUint64(&n) < c {
			time.Sleep(time.Millisecond * 20)
		}
		ln.Close()
	}()

	go func() {
		s.Serve(ln)
		ch <- struct{}{}
	}()

	for i := 0; i < clients; i++ {
		go func() {
			err := startFastClient(ln, &n, count)
			if err != nil {
				b.Fatal(err)
			}
		}()
	}

	select {
	case <-ch:
	case <-time.After(time.Second * 10):
		b.Fatal("timeout")
	}
}

func benchmarkHTTPServer(b *testing.B, clients int, count uint64) {
	var n uint64 = 0
	s := http.Server{
		Handler: &handler{b},
	}
	ch := make(chan struct{}, 1)
	ln := fasthttputil.NewInmemoryListener()

	go func() {
		c := count * uint64(clients)
		for atomic.LoadUint64(&n) < c {
			time.Sleep(time.Millisecond * 20)
		}
		ln.Close()
	}()

	go func() {
		s.Serve(ln)
		ch <- struct{}{}
	}()

	for i := 0; i < clients; i++ {
		go func() {
			err := startHTTPClient(ln, &n, count)
			if err != nil {
				b.Fatal(err)
			}
		}()
	}

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
		return
	}
	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			if err == io.EOF {
				break
			}
			h.b.Fatal(err)
		}
	}
	c.Close()
}

var text = []byte("Make fasthttp great again")

func startFastClient(ln *fasthttputil.InmemoryListener, n *uint64, count uint64) error {
	c, err := ln.Dial()
	if err != nil {
		return err
	}
	conn, err := Client(c, "http://localhost")
	if err != nil {
		return err
	}
	for i := uint64(0); i < count; i++ {
		_, err = conn.Write(text)
		if err != nil {
			break
		}
		atomic.AddUint64(n, 1)
	}
	return err
}

func startHTTPClient(ln *fasthttputil.InmemoryListener, n *uint64, count uint64) error {
	c, err := ln.Dial()
	if err != nil {
		return err
	}
	u := &url.URL{
		Scheme: "ws",
		Host:   "localhost",
	}
	conn, _, err := websocket.NewClient(c, u, nil, 0, 0)
	if err != nil {
		return err
	}
	for i := uint64(0); i < count; i++ {
		err = conn.WriteMessage(websocket.TextMessage, text)
		if err != nil {
			break
		}
		atomic.AddUint64(n, 1)
	}
	return err
}

func Benchmark1000ClientsPer10Message(b *testing.B) {
	benchmarkFastServer(b, 1000, 10)
}

func Benchmark1000ClientsPer100Message(b *testing.B) {
	benchmarkFastServer(b, 1000, 100)
}

func Benchmark1000ClientsPer1000Message(b *testing.B) {
	benchmarkFastServer(b, 1000, 1000)
}

func Benchmark1000HTTPClientsPer10Message(b *testing.B) {
	benchmarkHTTPServer(b, 1000, 10)
}

func Benchmark1000HTTPClientsPer100Message(b *testing.B) {
	benchmarkHTTPServer(b, 1000, 100)
}

func Benchmark1000HTTPClientsPer1000Message(b *testing.B) {
	benchmarkHTTPServer(b, 1000, 1000)
}

func Benchmark100MsgsPerConn(b *testing.B) {
	benchmarkFastServer(b, runtime.NumCPU(), 100)
}

func Benchmark1000MsgsPerConn(b *testing.B) {
	benchmarkFastServer(b, runtime.NumCPU(), 1000)
}

func Benchmark10000MsgsPerConn(b *testing.B) {
	benchmarkFastServer(b, runtime.NumCPU(), 10000)
}

func Benchmark100000MsgsPerConn(b *testing.B) {
	benchmarkFastServer(b, runtime.NumCPU(), 100000)
}

func Benchmark100MsgsHTTPPerConn(b *testing.B) {
	benchmarkHTTPServer(b, runtime.NumCPU(), 100)
}

func Benchmark1000MsgsHTTPPerConn(b *testing.B) {
	benchmarkHTTPServer(b, runtime.NumCPU(), 1000)
}

func Benchmark10000MsgsHTTPPerConn(b *testing.B) {
	benchmarkHTTPServer(b, runtime.NumCPU(), 10000)
}

func Benchmark100000MsgsHTTPPerConn(b *testing.B) {
	benchmarkHTTPServer(b, runtime.NumCPU(), 100000)
}
