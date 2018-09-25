package fastws

import (
	"testing"

	"github.com/valyala/fasthttp"
)

func TestParseExtensions(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.DisableNormalizing()
	ctx.Request.Header.AddBytesK(wsHeaderExtensions, "foo, bar; x=20; y=10")
	ctx.Request.Header.AddBytesK(wsHeaderExtensions, "foo2")

	exts := parseExtensions(ctx)
	if len(exts) == 0 {
		t.Fatal("no extensions detected")
	}
	if string(exts[0].key) != "foo" {
		t.Fatalf("%s <> foo", exts[0].key)
	}
	if string(exts[1].key) != "bar" {
		t.Fatalf("%s <> bar", exts[1].key)
	}
	if string(exts[1].params[0].key) != "x" &&
		string(exts[1].params[0].value) == "20" {
		t.Fatalf("%s <> x & %s <> 20", exts[1].params[0].key, exts[1].params[0].value)
	}
	if string(exts[1].params[1].key) != "y" &&
		string(exts[1].params[1].value) == "10" {
		t.Fatalf("%s <> x & %s <> 20", exts[1].params[1].key, exts[1].params[1].value)
	}
	if string(exts[2].key) != "foo2" {
		t.Fatalf("%s <> foo2", exts[2].key)
	}
}
