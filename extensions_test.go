package fastws

import (
	"bytes"
	"testing"

	"github.com/valyala/fasthttp"
)

func compareExtension(t *testing.T, ext, ext2 *extension) {
	if !bytes.Equal(ext2.key, ext.key) {
		t.Fatalf("bad key value: %s <> %s", ext2.key, ext.key)
	}
	for j := range ext2.params {
		if !bytes.Equal(ext2.params[j].key, ext.params[j].key) {
			t.Fatalf("bad key param value: %s <> %s", ext2.params[j].key, ext.params[j].key)
		}
		if !bytes.Equal(ext2.params[j].value, ext.params[j].value) {
			t.Fatalf("bad value param value: %s <> %s", ext2.params[j].value, ext.params[j].value)
		}
	}
}

func compareExtensions(t *testing.T, exts, ext2 []*extension) {
	for i := range ext2 {
		compareExtension(t, exts[i], ext2[i])
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
	if string(b) != "foo; x=10; y=20" {
		t.Fatalf("bad encoding \"%s\"", b)
	}

	ext2 := &extension{}
	ext2.parse(b)

	compareExtension(t, ext, ext2)
}

func TestParseBadExtensions(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.AddBytesK(wsHeaderExtensions, "a, b, c, d, e; f, g, h;")
	ctx.Request.Header.AddBytesK(wsHeaderExtensions, "a;p=1;b,a;")

	exts := parseExtensions(ctx)
	ext2 := []*extension{
		&extension{
			key: []byte("a"),
		},
		&extension{
			key: []byte("b"),
		},
		&extension{
			key: []byte("c"),
		},
		&extension{
			key: []byte("d"),
		},
		&extension{
			key: []byte("e"),
		},
		&extension{
			key: []byte("f"),
		},
		&extension{
			key: []byte("g"),
		},
		&extension{
			key: []byte("h"),
		},
		&extension{
			key: []byte("a"),
			params: []*parameter{
				&parameter{
					key:   []byte("p"),
					value: []byte("1"),
				},
			},
		},
		&extension{
			key: []byte("b"),
		},
		&extension{
			key: []byte("a"),
		},
	}
	compareExtensions(t, exts, ext2)
}

func TestParseExtensions(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.AddBytesK(wsHeaderExtensions, "foo, bar; x=20; y=10")
	ctx.Request.Header.AddBytesK(wsHeaderExtensions, "foo2")
	ctx.Request.Header.AddBytesK(wsHeaderExtensions, "foo3; a")
	ctx.Request.Header.AddBytesK(wsHeaderExtensions, "mux; max-channels=4; flow-control, deflate-stream")

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
		&extension{
			key: []byte("mux"),
			params: []*parameter{
				&parameter{
					key:   []byte("max-channels"),
					value: []byte("4"),
				},
			},
		},
		&extension{
			key: []byte("flow-control"),
		},
		&extension{
			key: []byte("deflate-stream"),
		},
	}
	compareExtensions(t, exts, ext2)
}
