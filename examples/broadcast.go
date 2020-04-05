// +build ignore
package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/buaazp/fasthttprouter"
	"github.com/dgrr/fastws"
	"github.com/valyala/fasthttp"
)

type Broadcaster struct {
	lck sync.Mutex
	cs  []*fastws.Conn
}

func (b *Broadcaster) Add(c *fastws.Conn) {
	b.lck.Lock()
	b.cs = append(b.cs, c)
	b.lck.Unlock()
}

func (b *Broadcaster) Start() {
	for {
		b.lck.Lock()
		for i := 0; i < len(b.cs); i++ {
			c := b.cs[i]
			_, err := c.WriteString("Message")
			if err != nil {
				b.cs = append(b.cs[:i], b.cs[i+1:]...)
				fmt.Println(len(b.cs))
				continue
			}
		}
		b.lck.Unlock()

		time.Sleep(time.Second)
	}
}

func main() {
	b := &Broadcaster{}
	router := fasthttprouter.New()
	router.GET("/", rootHandler)
	router.GET("/ws", fastws.Upgrade(func(c *fastws.Conn) {
		b.Add(c)
		for {
			_, _, err := c.ReadMessage(nil)
			if err != nil {
				if err == fastws.EOF {
					break
				}
				panic(err)
			}
		}
	}))
	go b.Start()

	server := fasthttp.Server{
		Handler: router.Handler,
	}
	go server.ListenAndServe(":8080")

	fmt.Println("Visit http://localhost:8080")

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	signal.Stop(sigCh)
	signal.Reset(os.Interrupt)
	server.Shutdown()
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
      var ws = new WebSocket("ws://localhost:8080/ws");
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
