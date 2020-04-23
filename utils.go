package fastws

import (
	"net/http"
	"reflect"
	"unsafe"

	"github.com/valyala/fasthttp"
)

// Upgrade returns a RequestHandler for fasthttp doing the upgrading process easier.
func Upgrade(handler RequestHandler) func(ctx *fasthttp.RequestCtx) {
	upgr := Upgrader{
		Handler:  handler,
		Compress: true,
	}
	return upgr.Upgrade
}

// NetUpgrade returns a RequestHandler for net/http doing the upgrading process easier.
func NetUpgrade(handler RequestHandler) func(http.ResponseWriter, *http.Request) {
	upgr := NetUpgrader{
		Handler:  handler,
		Compress: true,
	}
	return upgr.Upgrade
}

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func s2b(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
}

func equalsFold(b, s []byte) (equals bool) {
	n := len(b)
	equals = n == len(s)
	if equals {
		for i := 0; i < n; i++ {
			if equals = b[i]|0x20 == s[i]|0x20; !equals {
				break
			}
		}
	}
	return
}
