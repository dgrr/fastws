package fastws

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func BenchmarkRandKey(b *testing.B) {
	var bf []byte
	for i := 0; i < b.N; i++ {
		bf = makeRandKey(bf[:0])
	}
}

func TestDial(t *testing.T) {
	var text = []byte("Make fasthttp great again")
	var uri = "http://localhost:9843/"
	ln := fasthttputil.NewInmemoryListener()
	upgr := Upgrader{
		Origin: uri,
		Handler: func(conn *Conn) {
			for {
				_, b, err := conn.ReadMessage(nil)
				if err != nil {
					if err == EOF {
						break
					}
					panic(err)
				}
				if !bytes.Equal(b, text) {
					panic(fmt.Sprintf("%s <> %s", b, text))
				}
			}
		},
	}
	s := fasthttp.Server{
		Handler: upgr.Upgrade,
	}
	ch := make(chan struct{}, 1)
	go func() {
		s.Serve(ln)
		ch <- struct{}{}
	}()

	c, err := ln.Dial()
	if err != nil {
		t.Fatal(err)
	}

	conn, err := Client(c, uri)
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.Write(text)
	if err != nil {
		t.Fatal(err)
	}

	conn.Close()
	ln.Close()

	select {
	case <-ch:
	case <-time.After(time.Second * 5):
		t.Fatal("timeout")
	}
}

type hijackHandler struct {
	h func(c *Conn)
}

func (h *hijackHandler) sendResponseUpgrade(ctx *fasthttp.RequestCtx) {
	hkey := ctx.Request.Header.PeekBytes(wsHeaderKey)
	ctx.Response.SetStatusCode(fasthttp.StatusSwitchingProtocols)
	ctx.Response.Header.AddBytesKV(connectionString, upgradeString)
	ctx.Response.Header.AddBytesK(upgradeString, "websocket")
	ctx.Response.Header.AddBytesKV(wsHeaderAccept, makeKey(hkey, hkey))
	ctx.Response.Header.AddBytesK(wsHeaderProtocol, "13")
	ctx.Hijack(func(c net.Conn) {
		conn := acquireConn(c)
		conn.server = true
		h.h(conn)
		conn.Close()
	})
}

func TestDialLowerCaseWebsocketString(t *testing.T) {
	var text = []byte("Make fasthttp great again")
	var uri = "http://localhost:9844/"
	ln := fasthttputil.NewInmemoryListener()
	hfn := func(conn *Conn) {
		_, b, err := conn.ReadMessage(nil)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(b, text) {
			panic(fmt.Sprintf("%s <> %s", b, text))
		}
	}

	s := fasthttp.Server{
		Handler: (&hijackHandler{hfn}).sendResponseUpgrade,
	}
	ch := make(chan struct{}, 1)
	go func() {
		s.Serve(ln)
		ch <- struct{}{}
	}()

	c, err := ln.Dial()
	if err != nil {
		t.Fatal(err)
	}

	conn, err := Client(c, uri)
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.Write(text)
	if err != nil {
		t.Fatal(err)
	}

	conn.Close()
	ln.Close()

	select {
	case <-ch:
	case <-time.After(time.Second * 5):
		t.Fatal("timeout")
	}
}

func TestClientConcurrentWrite(t *testing.T) {
	n := 10000 // 10_000 not working for gofmt?
	var text = []byte("Make fasthttp great again")
	var uri = "http://localhost:9843/"
	ln := fasthttputil.NewInmemoryListener()
	upgr := Upgrader{
		Origin: uri,
		Handler: func(conn *Conn) {
			cnt := 0
			for {
				_, b, err := conn.ReadMessage(nil)
				if err != nil {
					if err == EOF {
						break
					}
					t.Fatal(err)
				}
				if string(b) != string(text) {
					t.Fatalf("%s <> %s", b, text)
				}
				cnt++
			}
			if cnt != n {
				t.Fatalf("Expected %d. Recv %d", n, cnt)
			}
		},
	}
	s := fasthttp.Server{
		Handler: upgr.Upgrade,
	}
	ch := make(chan struct{}, 1)
	go func() {
		s.Serve(ln)
		ch <- struct{}{}
	}()

	c, err := ln.Dial()
	if err != nil {
		t.Fatal(err)
	}

	conn, err := Client(c, uri)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var chs = make([]chan string, 16)
	for i := range chs {
		chs[i] = make(chan string, 1)
		wg.Add(1)
		go func(ch chan string) {
			defer wg.Done()

			for msg := range ch {
				_, err := conn.WriteString(msg)
				if err != nil {
					if err == EOF {
						break
					}
					panic(err)
				}
			}
		}(chs[i])
	}

loop:
	for i := 0; i < n; i++ {
		for j := 0; ; j++ {
			if j == len(chs) {
				j = 0
			}
			select {
			case chs[j] <- string(text):
				continue loop
			default:
			}
		}
	}
	for i := range chs {
		close(chs[i])
	}

	wg.Wait()
	conn.Close()
	ln.Close()

	select {
	case <-ch:
	case <-time.After(time.Second * 5):
		t.Fatal("timeout")
	}
}

func TestConnCloseWhileReading(t *testing.T) {
	var uri = "http://localhost:9843/"
	ln := fasthttputil.NewInmemoryListener()
	upgr := Upgrader{
		Origin: uri,
		Handler: func(conn *Conn) {
			go func() {
				for {
					_, _, err := conn.ReadMessage(nil)
					if err != nil {
						if err == EOF {
							break
						}
						panic(err)
					}
				}
			}()
			time.Sleep(time.Millisecond * 100)
			conn.Close()
		},
	}
	s := fasthttp.Server{
		Handler: upgr.Upgrade,
	}
	ch := make(chan struct{}, 1)
	go func() {
		s.Serve(ln)
		ch <- struct{}{}
	}()

	c, err := ln.Dial()
	if err != nil {
		t.Fatal(err)
	}

	conn, err := Client(c, uri)
	if err != nil {
		t.Fatal(err)
	}

	for {
		_, _, err := conn.ReadMessage(nil)
		if err != nil {
			break
		}
	}

	conn.Close()
	ln.Close()

	select {
	case <-ch:
	case <-time.After(time.Second * 2):
		t.Fatal("timeout")
	}
}
