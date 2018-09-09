package main

import (
	"fmt"
	"time"

	"github.com/buaazp/fasthttprouter"
	"github.com/dgrr/fastws"
	"github.com/valyala/fasthttp"
)

func main() {
	upgr := fastws.Upgrader{
		Handler: wsHandler,
	}
	router := fasthttprouter.New()
	router.GET("/", rootHandler)
	router.GET("/ws", upgr.Upgrade)

	fasthttp.ListenAndServe(":8080", router.Handler)
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
    <script>
			var ws = new WebSocket("ws://localhost:8080/ws");
      ws.onmessage = function(e) {
				console.log(e.data)
				ws.send(e.data)
      }
    </script>
  </body>
</html>`)
}

func wsHandler(conn *fastws.Conn) {
	for i := 0; i < 20; i++ {
		go doRead(conn)
		go doWrite(conn)
	}
	go doRead(conn)
	doWrite(conn)
}

func doRead(conn *fastws.Conn) {
	for i := 0; i < 100; i++ {
		a := make([]byte, 10)
		n, err := conn.Read(a)
		if err != nil {
			fmt.Println(err)
			break
		}
		fmt.Printf("%d: %s\n", n, a)
	}
	conn.Close()
}

func doWrite(conn *fastws.Conn) {
	for {
		time.Sleep(time.Millisecond * 100)
		text := fmt.Sprintf("%v", time.Now().Unix())
		fmt.Fprintf(conn, text)
	}
}
