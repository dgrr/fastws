package main

import (
	"context"
	"net/http"

	nyws "nhooyr.io/websocket"
)

type handler struct{}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := nyws.Accept(w, r, &nyws.AcceptOptions{
		CompressionMode: nyws.CompressionDisabled, // disable compression to be fair
	})
	if err != nil {
		return
	}

	ctx := context.TODO()

	for {
		t, b, err := c.Read(ctx)
		if err != nil {
			break
		}
		c.Write(ctx, t, b)
	}
	c.Close(nyws.StatusNormalClosure, "")
}

func main() {
	s := http.Server{
		Addr:    ":8081",
		Handler: &handler{},
	}
	s.ListenAndServe()
}
