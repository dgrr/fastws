package main

import (
	"fmt"

	"github.com/dgrr/fastws"
	"github.com/valyala/fasthttp"
)

func main() {
	fasthttp.ListenAndServe(":9000", fastws.Upgrade(wsHandler))
}

func wsHandler(conn *fastws.Conn) {
	var err error
	var fr = fastws.AcquireFrame()
	conn.MaxPayloadSize = 65536
	var accp []byte // accumulated payload
	for {
		accp, err = conn.ReadFull(accp[:0], fr)
		if err != nil {
			break
		}

		_, err = conn.WriteFrame(fr)
		if err != nil {
			break
		}
	}

	fmt.Printf("Closed connection: %v\n", err)
}
