package main

import (
	"fmt"
	"log"
	"time"

	router "github.com/dgrr/fasthttprouter"
	"github.com/valyala/fasthttp"

	"github.com/dgrr/fastws"
)

func main() {
	// Configure websocket upgrader.
	upgr := fastws.Upgrader{
		UpgradeHandler: tokenVerify,
		Handler:        websocketHandler,
	}

	// Configure router handler.
	r := router.New()
	r.GET("/loginHandler", loginHandler)
	r.GET("/ws", upgr.Upgrade)

	server := fasthttp.Server{
		Handler: r.Handler,
	}
	go server.ListenAndServe(":8080")

	startClient("ws://:8080/ws", "http://localhost:8080/loginHandler")

	select {}
}

func websocketHandler(c *fastws.Conn) {
	c.WriteString("Hello world")
	for {
		_, msg, err := c.ReadMessage(nil)
		if err != nil {
			fmt.Printf("ERROR %v\n", err)
			break
		}
		log.Printf("Readed %s\n", msg)
		_, err = c.Write(msg)
		if err != nil {
			fmt.Printf("ERROR %v\n", err)
			break
		}
	}
	c.Close()
}

func startClient(urlws, urlset string) {
	// c, err := fastws.Dial(urlws)
	// if err == nil {
	// 	panic("connected")
	// }

	req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(urlset)

	err := fasthttp.Do(req, resp)
	checkErr(err)

	if resp.StatusCode() == fasthttp.StatusOK {
		token := resp.Body()

		req.Reset()
		req.Header.SetBytesKV(AuthHeader, token)

		conn, er1 := fastws.DialWithHeaders(urlws, req)
		checkErr(er1)
		defer conn.Close()

		log.Println("Connected")
		conn.WriteString("Hello")

		var msg []byte
		for i := 0; i < 50; i++ {
			_, msg, err := conn.ReadMessage(msg[:0])
			if err != nil {
				break
			}
			log.Printf("Client: %s\n", msg)
			conn.WriteString(time.Now().String())
			time.Sleep(time.Second)
		}

	}
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
