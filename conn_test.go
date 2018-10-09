package fastws

import (
	"bufio"
	"fmt"
	"sync"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func configureServer(t *testing.T) (*fasthttp.Server, *fasthttputil.InmemoryListener) {
	ln := fasthttputil.NewInmemoryListener()
	s := &fasthttp.Server{
		Handler: Upgrade(func(conn *Conn) {
			m, b, err := conn.ReadMessage(nil)
			if err != nil {
				t.Fatal(err)
			}
			if m != ModeText {
				t.Fatal("Unexpected code: Not ModeText")
			}

			if string(b) != "Hello" {
				t.Fatalf("Unexpected message: %s<>Hello", b)
			}

			conn.WriteString("Hello2")

			fr, err := conn.NextFrame()
			if err != nil {
				t.Fatal(err)
			}
			if !fr.IsPing() {
				t.Fatal("Unexpected message: no ping")
			}
			err = conn.SendCode(CodePong, StatusNone, nil)
			if err != nil {
				t.Fatal(err)
			}

			_, b, err = conn.ReadMessage(nil)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != "Hello world" {
				t.Fatalf("%s <> Hello world", b)
			}
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
		go s.Serve(ln)
		<-ch
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
	fr.SetContinuation()
	fr.SetPayload([]byte("Hello"))
	_, err = conn.WriteFrame(fr)
	if err != nil {
		t.Fatal(err)
	}
	fr.Reset()

	fr.SetText()
	fr.SetFin()
	fr.SetPayload([]byte(" world"))
	if err != nil {
		t.Fatal(err)
	}

	ch <- struct{}{}
}

func handleConcurrentRead(conn *Conn) (err error) {
	var b []byte
	for {
		_, b, err = conn.ReadMessage(b)
		if err != nil {
			if err == EOF {
				err = nil
			}
			break
		}
		if string(b) != textToSend {
			err = fmt.Errorf("%s <> %s", b, textToSend)
			break
		}
	}
	return
}

var textToSend = "hello"

func writeAndReadConcurrently(conn *Conn) (err error) {
	for i := 0; i < 100; i++ {
		_, err = conn.WriteString(textToSend)
		if err != nil {
			if err == EOF {
				err = nil
			}
			break
		}
	}
	if err == nil {
		conn.Close("")
	}
	return
}

func TestReadConcurrently(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()
	s := fasthttp.Server{
		Handler: Upgrade(func(conn *Conn) {
			var wg sync.WaitGroup
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					err := handleConcurrentRead(conn)
					if err != nil {
						panic(err)
					}
					wg.Done()
				}()
			}
			wg.Wait()
		}),
	}
	go s.Serve(ln)

	var wg sync.WaitGroup
	conn := openConn(t, ln)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			err := writeAndReadConcurrently(conn)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	ln.Close()
}
