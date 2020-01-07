package fastws

import (
	"bytes"
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
			_, b, err := conn.ReadMessage(nil)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(b, text) {
				t.Fatalf("%s <> %s", b, text)
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
