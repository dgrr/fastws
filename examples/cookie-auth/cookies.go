package main

import (
	"bytes"
	"log"
	"time"

	router "github.com/dgrr/fasthttprouter"
	"github.com/valyala/fasthttp"

	"github.com/dgrr/fastws"
)

func main() {
	// Configure websocket upgrader.
	upgr := fastws.Upgrader{
		UpgradeHandler: checkCookies,
		Handler:        websocketHandler,
	}

	// Configure router handler.
	r := router.New()
	r.GET("/set", setCookieHandler)
	r.GET("/ws", upgr.Upgrade)

	server := fasthttp.Server{
		Handler: r.Handler,
	}
	go server.ListenAndServe(":8080")

	startClient("ws://:8080/ws", "http://localhost:8080/set")
}

func websocketHandler(c *fastws.Conn) {
	c.WriteString("Hello world")
	_, msg, err := c.ReadMessage(nil)
	if err != nil {
		panic(err)
	}
	log.Printf("Readed %s\n", msg)
	c.Close()
}

var (
	cookieKey   = []byte("cookiekey")
	cookieValue = []byte("thisisavalidcookievalue")
)

func checkCookies(ctx *fasthttp.RequestCtx) bool {
	cookie := ctx.Request.Header.CookieBytes(cookieKey)
	if bytes.Equal(cookie, cookieValue) {
		return true
	}
	ctx.Error("You don't have a cookie D:", fasthttp.StatusBadRequest)
	return false
}

func setCookieHandler(ctx *fasthttp.RequestCtx) {
	setCookieWithTimeout(ctx, time.Time{})
}

func delCookieHandler(ctx *fasthttp.RequestCtx) {
	setCookieWithTimeout(ctx, time.Now())
}

func setCookieWithTimeout(ctx *fasthttp.RequestCtx, t time.Time) {
	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)

	cookie.SetKeyBytes(cookieKey)
	cookie.SetValueBytes(cookieValue)

	if !t.IsZero() {
		cookie.SetExpire(t)
	}

	ctx.Response.Header.SetCookie(cookie)
}

func startClient(urlws, urlset string) {
	c, err := fastws.Dial(urlws)
	if err == nil {
		panic("connected")
	}

	req, res := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(urlset)

	err = fasthttp.Do(req, res)
	checkErr(err)

	cookie.SetKeyBytes(cookieKey)
	if !res.Header.Cookie(cookie) {
		panic("cookie not found in response")
	}
	req.Reset()
	req.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())

	c, err = fastws.DialWithHeaders(urlws, req)
	checkErr(err)
	defer c.Close()

	log.Println("Connected")

	_, msg, err := c.ReadMessage(nil)
	checkErr(err)

	log.Printf("%s readed\n", msg)
	c.WriteString("Hello xdxd")
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
