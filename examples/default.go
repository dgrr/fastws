package main

import (
	"fmt"
	"time"

	"github.com/buaazp/fasthttprouter"
	"github.com/dgrr/fastws"
	"github.com/valyala/fasthttp"
)

func main() {
	router := fasthttprouter.New()
	router.GET("/", rootHandler)
	router.GET("/ws", fastws.Upgrade(wsHandler))

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
	go func() {
		for i := 0; i < 10; i++ {
			a := make([]byte, 10)
			n, err := conn.Read(a)
			if err != nil {
				fmt.Println(err)
				break
			}
			fmt.Printf("%d: %s\n", n, a)
		}
		conn.Close()
	}()

	for {
		time.Sleep(time.Second)
		text := fmt.Sprintf("%v", time.Now().Unix())
		_, err := fmt.Fprintf(conn, text)
		if err != nil {
			fmt.Println(err)
			break
		}
	}
}
