package main

import (
	"fmt"

	"github.com/dgrr/fastws"
	"github.com/valyala/fasthttp"
)

func main() {
	s := fasthttp.Server{
		Handler: fastws.Upgrade(func(c *fastws.Conn) {
			var (
				bf  []byte
				err error
				m   fastws.Mode
			)

			c.ReadTimeout = 0
			c.WriteTimeout = 0

			for {
				m, bf, err = c.ReadMessage(bf[:0])
				if err != nil {
					break
				}

				c.WriteMessage(m, bf)
			}

			c.Close()
		}),
	}
	fmt.Println(s.ListenAndServe(":8081"))
}
