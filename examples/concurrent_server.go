//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/buaazp/fasthttprouter"
	"github.com/dgrr/fastws"
	"github.com/valyala/fasthttp"
)

// This code will show you something you cannot do with other libraries
// MAGIC!!1!!

func main() {
	router := fasthttprouter.New()
	router.GET("/", rootHandler)
	router.GET("/ws", fastws.Upgrade(wsHandler))

	server := fasthttp.Server{
		Handler: router.Handler,
	}
	go func() {
		if err := server.ListenAndServe(":8081"); err != nil {
			log.Fatalln(err)
		}
	}()

	fmt.Println("Visit http://localhost:8081")

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	signal.Stop(sigCh)
	signal.Reset(os.Interrupt)
	server.Shutdown()
}

func wsHandler(conn *fastws.Conn) {
	fmt.Printf("Opened connection\n")

	conn.WriteString("Hello")

	var msg []byte
	var wg sync.WaitGroup
	mch := make(chan string, 128)

	wg.Add(2)
	go func() {
		defer wg.Done()
		defer close(mch)

		var err error
		for {
			_, msg, err = conn.ReadMessage(msg[:0])
			if err != nil {
				if err != fastws.EOF {
					fmt.Fprintf(os.Stderr, "error reading message: %s\n", err)
				}
				break
			}
			mch <- string(msg)
		}
	}()

	go func() {
		defer wg.Done()
		for m := range mch {
			_, err := conn.WriteString(m)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error writing message: %s\n", err)
				break
			}
			time.Sleep(time.Millisecond * 100)
		}
	}()

	wg.Wait()

	fmt.Printf("Closed connection\n")
}

func rootHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/html")
	fmt.Fprintln(ctx, `<!DOCTYPE html>
<html>
  <head>
    <meta charset="UTF-8"/>
    <title>Sample of websocket with Golang</title>
  </head>
  <body>
		<div id="text"></div>
    <script>
      var ws = new WebSocket("ws://localhost:8081/ws");
      ws.onmessage = function(e) {
				var d = document.createElement("div");
        d.innerHTML = e.data;
				ws.send(e.data);
        document.getElementById("text").appendChild(d);
      }
			ws.onclose = function(e){
				var d = document.createElement("div");
				d.innerHTML = "CLOSED";
        document.getElementById("text").appendChild(d);
			}
    </script>
  </body>
</html>`)
}
