package main

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

type handler struct{}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}

	for {
		t, msg, err := c.ReadMessage()
		if err != nil {
			break
		}
		c.WriteMessage(t, msg)
	}
	c.Close()
}

func main() {
	s := http.Server{
		Addr:    ":8081",
		Handler: &handler{},
	}
	s.ListenAndServe()
}
