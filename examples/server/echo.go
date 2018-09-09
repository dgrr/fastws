package main

import (
	"fmt"
	"os"
	"os/signal"

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
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt)

	go handleConn(conn)
	<-sigCh
	signal.Stop(sigCh)
	signal.Reset(os.Interrupt)

	conn.Close(nil)
}

func handleConn(conn *fastws.Conn) {
	fr := fastws.AcquireFrame()
	defer fastws.ReleaseFrame(fr)

	for {
		_, err := conn.ReadFrame(fr)
		if err != nil {
			break
		}
		if fr.IsMasked() {
			fr.Unmask()    // unmask/decode payload content
			fr.UnsetMask() // delete mask bit to be sended from the server
		}
		conn.WriteFrame(fr)
		fr.Reset()
	}
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
