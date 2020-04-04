package fastws

import (
	"bytes"
	"fmt"
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

func TestClientConcurrentWrite(t *testing.T) {
	n := 10_000
	var text = "Make fasthttp great again"
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
				if string(b) != text {
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
	defer conn.Close()

	var wg sync.WaitGroup
	var chs = make([]chan string, 16)
	for i := range chs {
		chs[i] = make(chan string, 1)
		wg.Add(1)
		go func(ch chan string) {
			defer wg.Done()

			for msg := range ch {
				_, err = conn.WriteString(msg)
				if err != nil {
					if err == EOF {
						break
					}
					t.Fatal(err)
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
			case chs[j] <- text:
				continue loop
			default:
			}
		}
	}
	for i := range chs {
		close(chs[i])
	}

	wg.Wait()
	ln.Close()

	select {
	case <-ch:
	case <-time.After(time.Second * 5):
		t.Fatal("timeout")
	}
}


func TestConnCloseWhileReading(t *testing.T) {
	var uri = "http://localhost:9843/"
	var text = "Make fasthttp great again"
	ln := fasthttputil.NewInmemoryListener()
	upgr := Upgrader{
		Origin: uri,
		Handler: func(conn *Conn) {
			go func() {
				for {
					_, b, err := conn.ReadMessage(nil)
					if err != nil {
						if err == EOF {
							break
						}
						panic(err)
					}
					if string(b) != text {
						t.Fatalf("%s <> %s", b, text)
					}
				}
			}()
			time.Sleep(time.Millisecond*100)
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

	go func() {
		for {
			_, _, err := conn.ReadMessage(nil)
			if err != nil {
				break
			}
		}
	}()

	for {
		_, err := conn.WriteString(text)
		if err != nil {
			if err == EOF {
				break
			}
			t.Fatal(err)
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