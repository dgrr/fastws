package main

import (
	"log"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/dgrr/fastws"
)

func main() {
	go startServer(":8080")

	conn, err := fastws.Dial("ws://localhost:8080/echo")
	if err != nil {
		log.Fatalln(err)
	}
	conn.WriteString("Hello")

	var msg []byte
	for i := 0; i < 5; i++ {
		_, msg, err = conn.ReadMessage(msg[:0])
		if err != nil {
			break
		}
		log.Printf("Client: %s\n", msg)
		conn.Write(msg)
		time.Sleep(time.Second)
	}
	conn.Close()
}

func echo(c *fastws.Conn) {
	defer c.Close()

	var msg []byte
	var err error
	for {
		_, msg, err = c.ReadMessage(msg[:0])
		if err != nil {
			break
		}
		log.Printf("Server: %s\n", msg)

		_, err = c.Write(msg)
		if err != nil {
			break
		}
	}
}

func startServer(addr string) {
	if err := fasthttp.ListenAndServe(addr, fastws.Upgrade(echo)); err != nil {
		log.Fatalln(err)
	}
}
