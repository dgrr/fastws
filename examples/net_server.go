//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/dgrr/fastws"
)

func main() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/ws", fastws.NetUpgrade(wsHandler))

	go http.ListenAndServe(":8080", nil)

	fmt.Println("Visit http://localhost:8080")

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	signal.Stop(sigCh)
	signal.Reset(os.Interrupt)
}

func wsHandler(conn *fastws.Conn) {
	fmt.Printf("Opened connection\n")

	conn.WriteString("Hello")

	var msg []byte
	var err error
	for {
		_, msg, err = conn.ReadMessage(msg[:0])
		if err != nil {
			if err != fastws.EOF {
				fmt.Fprintf(os.Stderr, "error reading message: %s\n", err)
			}
			break
		}
		time.Sleep(time.Second)

		_, err = conn.Write(msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error writing message: %s\n", err)
			break
		}
	}

	fmt.Printf("Closed connection\n")
}

func rootHandler(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Add("Content-Type", "text/html")
	fmt.Fprintln(resp, `<!DOCTYPE html>
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
