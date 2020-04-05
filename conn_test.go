package fastws

import (
	"bufio"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func configureServer(t *testing.T) (*fasthttp.Server, *fasthttputil.InmemoryListener) {
	ln := fasthttputil.NewInmemoryListener()
	s := &fasthttp.Server{
		Handler: Upgrade(func(conn *Conn) {
			m, b, err := conn.ReadMessage(nil)
			if err != nil {
				panic(err)
			}
			if m != ModeText {
				panic("Unexpected code: Not ModeText")
			}

			if string(b) != "Hello" {
				panic(fmt.Sprintf("Unexpected message: %s<>Hello", b))
			}

			conn.WriteString("Hello2")

			fr, err := conn.NextFrame()
			if err != nil {
				panic(err)
			}
			if !fr.IsPing() {
				panic("Unexpected message: no ping")
			}
			err = conn.SendCode(CodePong, StatusNone, nil)
			if err != nil {
				panic(err)
			}

			_, b, err = conn.ReadMessage(b[:0])
			if err != nil {
				panic(err)
			}
			if string(b) != "Hello world" {
				panic(fmt.Sprintf("%s <> Hello world", b))
			}
			conn.CloseString("Bye")
		}),
	}
	return s, ln
}

func openConn(t *testing.T, ln *fasthttputil.InmemoryListener) *Conn {
	c, err := ln.Dial()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Fprintf(c, "GET / HTTP/1.1\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\n\r\n")

	br := bufio.NewReader(c)
	var res fasthttp.Response
	err = res.Read(br)
	if err != nil {
		t.Fatal(err)
	}

	conn := acquireConn(c)
	return conn
}

func TestReadFrame(t *testing.T) {
	s, ln := configureServer(t)
	ch := make(chan struct{})
	go func() {
		s.Serve(ln)
		ch <- struct{}{}
	}()

	conn := openConn(t, ln)
	conn.WriteString("Hello")

	m, b, err := conn.ReadMessage(nil)
	if err != nil {
		t.Fatal(err)
	}
	if m != ModeText {
		t.Fatal("Unexpected code: Not ModeText")
	}

	if string(b) != "Hello2" {
		t.Fatalf("Unexpected message: %s<>Hello2", b)
	}

	err = conn.SendCode(CodePing, StatusNone, nil)
	if err != nil {
		t.Fatal(err)
	}

	fr, err := conn.NextFrame()
	if err != nil {
		t.Fatal(err)
	}
	if !fr.IsPong() {
		t.Fatal("Unexpected message: no pong")
	}
	fr.Reset()

	fr.SetText()
	fr.SetPayload([]byte("Hello"))
	fr.Mask()
	_, err = conn.WriteFrame(fr)
	if err != nil {
		t.Fatal(err)
	}
	fr.Reset()

	fr.SetContinuation()
	fr.SetFin()
	fr.SetPayload([]byte(" world"))
	fr.Mask()
	_, err = conn.WriteFrame(fr)
	if err != nil {
		t.Fatal(err)
	}

	fr, err = conn.NextFrame()
	if err != nil {
		t.Fatal(err)
	}
	if !fr.IsClose() {
		t.Fatal("Unexpected frame: no close")
	}
	p := fr.Payload()
	if string(p) != "Bye" {
		t.Fatalf("Unexpected payload: %v <> Bye", p)
	}
	if fr.Status() != StatusNone {
		t.Fatalf("Status unexpected: %d <> %d", fr.Status(), StatusNone)
	}
	err = conn.SendCode(fr.Code(), fr.Status(), nil)
	if err != nil {
		t.Fatal(err)
	}

	ln.Close()
	<-ch
}

func handleConcurrentRead(conn *Conn) (err error) {
	var b []byte
	for {
		_, b, err = conn.ReadMessage(b[:0])
		if err != nil {
			if err == EOF {
				err = nil
			}
			return err
		}
		if string(b) != textToSend {
			err = fmt.Errorf("%s <> %s", b, textToSend)
			break
		}
	}

	return err
}

var textToSend = "hello"

func writeConcurrently(conn *Conn) (err error) {
	for i := 0; i < 10; i++ {
		_, err = conn.WriteString(textToSend)
		if err != nil {
			break
		}
	}
	return
}

func TestReadConcurrently(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()
	s := fasthttp.Server{
		Handler: Upgrade(func(conn *Conn) {
			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					err := handleConcurrentRead(conn)
					if err != nil {
						panic(err)
					}
				}()
			}
			wg.Wait()
		}),
	}
	go s.Serve(ln)

	var wg sync.WaitGroup
	conn := openConn(t, ln)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := writeConcurrently(conn)
			if err != nil {
				panic(err)
			}
		}()
	}
	wg.Wait()
	conn.Close()
	s.Shutdown()
	ln.Close()
}

func TestCloseWhileReading(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()
	s := fasthttp.Server{
		Handler: Upgrade(func(conn *Conn) {
			go func() {
				_, _, err := conn.ReadMessage(nil)
				if err != nil {
					if err == EOF {
						return
					}
					panic(err)
				}
			}()
			time.Sleep(time.Millisecond * 100)
			conn.Close()
		}),
	}
	go s.Serve(ln)

	conn := openConn(t, ln)
	conn.Close()
	s.Shutdown()
	ln.Close()
}

func TestUserValue(t *testing.T) {
	var uri = "http://localhost:9843/"
	var text = "Hello user!!"
	ln := fasthttputil.NewInmemoryListener()
	upgr := Upgrader{
		Origin: uri,
		Handler: func(conn *Conn) {
			v := conn.UserValue("custom")
			if v == nil {
				t.Fatal("custom is nil")
			}
			conn.WriteString(v.(string))
		},
	}
	s := fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			ctx.SetUserValue("custom", text)
			upgr.Upgrade(ctx)
		},
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

	_, b, err := conn.ReadMessage(nil)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != text {
		t.Fatalf("client expecting %s. Got %s", text, b)
	}

	conn.Close()
	ln.Close()

	select {
	case <-ch:
	case <-time.After(time.Second * 5):
		t.Fatal("timeout")
	}
}
