package main

import (
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

type handler struct{}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, br, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return
	}

	for {
		b, t, err := wsutil.ReadClientData(br)
		if err != nil {
			break
		}
		wsutil.WriteServerMessage(br, t, b)
		br.Flush()
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
