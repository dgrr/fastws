package main

import (
	"fmt"
	"time"

	"github.com/dgrr/fastws"
	"github.com/valyala/fasthttp"
)

func main() {
	go startServer(":8080")

	conn, err := fastws.Dial("ws://localhost:8080/echo")
	if err != nil {
		fmt.Println(err)
	}
	conn.WriteString("Hello")

	var msg []byte
	for i := 0; i < 5; i++ {
		_, msg, err = conn.ReadMessage(msg[:0])
		if err != nil {
			break
		}
		fmt.Printf("Client: %s\n", msg)
		conn.Write(msg)
		time.Sleep(time.Second)
	}
	conn.Close("Bye")
}

func echo(c *fastws.Conn) {
	defer c.Close("Bye")

	var msg []byte
	var err error
	for {
		_, msg, err = c.ReadMessage(msg)
		if err != nil {
			break
		}
		fmt.Printf("Server: %s\n", msg)

		_, err = c.Write(msg)
		if err != nil {
			break
		}
	}
}

func startServer(addr string) {
	fasthttp.ListenAndServe(addr, fastws.Upgrade(echo))
}
