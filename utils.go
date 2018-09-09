package fastws

import (
	"reflect"
	"unsafe"

	"github.com/valyala/fasthttp"
)

// Dial ...
func Dial(url string) (*Conn, error) {
	return nil, nil
}

// Upgrade returns a RequestHandler for fasthttp resuming upgrading process.
func Upgrade(handler RequestHandler) fasthttp.RequestHandler {
	upgr := Upgrader{
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
