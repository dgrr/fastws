package fastws

import (
	"bytes"
	"testing"

	"github.com/valyala/fasthttp"
)

func compareExtensions(t *testing.T, exts, ext2 []*extension) {
	for i := range ext2 {
		if !bytes.Equal(ext2[i].key, exts[i].key) {
			t.Fatalf("bad key value: %s <> %s", ext2[i].key, exts[i].key)
		}
		for j := range ext2[i].params {
			if !bytes.Equal(ext2[i].params[j].key, exts[i].params[j].key) {
				t.Fatalf("bad key param value: %s <> %s", ext2[i].params[j].key, exts[i].params[j].key)
			}
			if !bytes.Equal(ext2[i].params[j].value, exts[i].params[j].value) {
				t.Fatalf("bad value param value: %s <> %s", ext2[i].params[j].value, exts[i].params[j].value)
			}
		}
	}
}

func TestBuildExtension(t *testing.T) {
	ext := &extension{ // foo; x=10
		key: []byte("foo"),
		params: []*parameter{
			&parameter{
				key:   []byte("x"),
				value: []byte("10"),
			},
			&parameter{
				key:   []byte("y"),
				value: []byte("20"),
			},
		},
	}
	b := ext.build(nil)
	if len(b) == 0 {
		t.Fatal("size = 0")
	}
	if string(b) != "foo; x=10; y=20" {
		t.Fatalf("bad encoding \"%s\"", b)
	}
}

func TestParseExtensions(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.DisableNormalizing()
	ctx.Request.Header.AddBytesK(wsHeaderExtensions, "foo, bar; x=20; y=10")
	ctx.Request.Header.AddBytesK(wsHeaderExtensions, "foo2")
	ctx.Request.Header.AddBytesK(wsHeaderExtensions, "foo3; a")

	exts := parseExtensions(ctx)
	if len(exts) == 0 {
		t.Fatal("no extensions detected")
	}
	ext2 := []*extension{
		&extension{
			key: []byte("foo"),
		},
		&extension{
			key: []byte("bar"),
			params: []*parameter{
				&parameter{
					key:   []byte("x"),
					value: []byte("20"),
				},
				&parameter{
					key:   []byte("y"),
					value: []byte("10"),
				},
			},
		},
		&extension{
			key: []byte("foo2"),
		},
		&extension{
			key: []byte("foo3"),
			params: []*parameter{
				&parameter{
					key: []byte("a"),
				},
			},
		},
	}
	compareExtensions(t, exts, ext2)
}
