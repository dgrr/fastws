package fastws

import (
	"bytes"
	"sync"

	"github.com/valyala/fasthttp"
)

type extension struct {
	key    []byte
	params []*parameter
}

func (ext *extension) Reset() {
	ext.key = ext.key[:0]
	for i := range ext.params {
		paramPool.Put(ext.params[i])
	}
	ext.params = ext.params[:0]
}

var extPool = sync.Pool{
	New: func() interface{} {
		return &extension{}
	},
}

func releaseExtensions(exts []*extension) {
	for _, ext := range exts {
		ext.Reset()
		extPool.Put(ext)
	}
}

type parameter struct {
	key   []byte
	value []byte
}

var paramPool = sync.Pool{
	New: func() interface{} {
		return &parameter{}
	},
}

func skipWhiteSpaces(b []byte) []byte {
	i := 0
	for i = range b {
		if b[i] != ' ' {
			break
		}
	}
	return b[i:]
}

func (ext *extension) build(b []byte) []byte {
	b = append(b[:0], ext.key...)
	for _, param := range ext.params {
		b = append(b, ';', ' ')
		b = append(b, param.key...)
		b = append(b, '=')
		b = append(b, param.value...)
	}
	return b
}

func (ext *extension) parse(b []byte) []byte { // TODO: Make test
	var i int
	var c byte
	var cc byte // last char
	var prtr *parameter
	// extension = key, key; parameters=value
	ext.Reset()
floop:
	for {
		c, i, b = nextChar(b)
		if len(b) == 0 {
			break
		}

		switch c {
		case ',':
			if len(ext.key) > 0 { // another extension
				break floop
			}
			ext.key = append(ext.key[:0], b[:i]...)
			b = b[i+1:]
			break floop
		case ';':
			if len(ext.key) == 0 {
				ext.key = append(ext.key[:0], b[:i]...)
			} else if cc == '=' {
				if prtr != nil {
					ext.appendValue(prtr, b[:i])
					prtr = nil
				}
			} else {
				ext.appendKey(b[:i])
			}
		case '=':
			if len(ext.key) > 0 {
				prtr = paramPool.Get().(*parameter)
				prtr.key = append(prtr.key[:0], b[:i]...)
			}
		default:
			if len(ext.key) == 0 {
				ext.key = append(ext.key[:0], b...)
			} else if prtr != nil {
				ext.appendValue(prtr, b)
				prtr = nil
			} else {
				ext.appendKey(b)
			}
			i = len(b)
		}
		if i == len(b) {
			b = b[:0]
			break floop
		}
		b, cc = b[i+1:], c
	}
	return b
}

func (ext *extension) appendKey(key []byte) {
	p := paramPool.Get().(*parameter)
	p.key = append(p.key[:0], key...)
	ext.params = append(ext.params, p)
}

func (ext *extension) appendValue(prtr *parameter, value []byte) {
	prtr.value = append(prtr.value[:0], value...)
	ext.params = append(ext.params, prtr)
}

var chars = []byte{';', ',', '='}

func nextChar(b []byte) (byte, int, []byte) {
	var c byte
	var nn, i int
	b = skipWhiteSpaces(b)
	if len(b) > 0 {
		for n := 0; n < len(chars); n++ {
			nn = bytes.IndexByte(b, chars[n])
			if nn != -1 && (nn < i || i == 0) {
				c, i = chars[n], nn
			}
		}
	}
	return c, i, b
}

func parseExtensions(ctx *fasthttp.RequestCtx) []*extension {
	var exts []*extension
	ctx.Request.Header.VisitAll(func(k, v []byte) {
		if equalFold(k, wsHeaderExtensions) {
			for len(v) > 0 {
				ext := extPool.Get().(*extension)
				v = ext.parse(v)
				exts = append(exts, ext)
			}
		}
	})
	return exts
}
