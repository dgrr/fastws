package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dgrr/fastws"
	"github.com/gorilla/websocket"
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
		_, msg, err = conn.ReadMessage(msg)
		if err != nil {
			break
		}
		fmt.Printf("Client: %s\n", msg)
		conn.Write(msg)
		time.Sleep(time.Second)
	}
	conn.Close("Bye")
}

var upgrader = websocket.Upgrader{} // use default options

func echo(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("upgrade:", err)
		return
	}
	defer c.Close()

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			fmt.Println("read:", err)
			break
		}
		fmt.Printf("Server: %s\n", message)

		err = c.WriteMessage(mt, message)
		if err != nil {
			fmt.Println("write:", err)
			break
		}
	}
}

func startServer(addr string) {
	http.HandleFunc("/echo", echo)
	http.ListenAndServe(addr, nil)
}
