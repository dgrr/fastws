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

func wsHandler(conn *fastws.Conn) {
	for i := 0; i < 10; i++ {
		n, err := fmt.Fprintf(conn, "%v\n", time.Now().Unix())
		if err != nil {
			break
		}
		fmt.Printf("Sended %d bytes\n", n)
		time.Sleep(time.Second)
	}
	conn.Close()
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
