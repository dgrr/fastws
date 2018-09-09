# Fastws

Websocket library for [fasthttp](https://github.com/valyala/fasthttp).

Example
-------

```go
package main

import (
  "fmt"
  "time"

  "github.com/buaazp/fasthttprouter"
  "github.com/dgrr/fastws"
  "github.com/valyala/fasthttp"
)

func main() {
  router := fasthttprouter.New()
  router.GET("/", rootHandler)
  router.GET("/ws", fastws.Upgrade(wsHandler))

  fasthttp.ListenAndServe(":8080", router.Handler)
}

func wsHandler(conn *fastws.Conn) {
  for i := 0; i < 10; i++ {
    n, err := fmt.Fprintf(conn, "%v\n", time.Now().Unix())
    if err != nil {
      break
    }
    fmt.Printf("Sended %d bytes\n", n)
    time.Sleep(time.Second)
  }
  conn.Close()
}
```
