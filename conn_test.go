package fastws

import (
	"bufio"
	"fmt"
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

func TestReadFrame(t *testing.T) {
	s, ln := configureServer(t)
	ch := make(chan struct{})
	go func() {
		go s.Serve(ln)
		<-ch
	}()

	c, err := ln.Dial()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Fprintf(c, "GET / HTTP/1.1\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\n\r\n")

	br := bufio.NewReader(c)
	var res fasthttp.Response
	res.Read(br)

	conn := acquireConn(c)
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
