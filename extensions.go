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

var extPool = sync.Pool{
	New: func() interface{} {
		return &extension{}
	},
}

func releaseExtensions(exts []*extension) {
	for _, ext := range exts {
		for i := range ext.params {
			paramPool.Put(ext.params[i])
		}
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

func skipWhiteSpace(b []byte) []byte {
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
	// extension = key, key; parameters=value
	for {
		b = skipWhiteSpace(b)
		if len(b) == 0 {
			break
		}

		i = bytes.IndexByte(b, ',')
		if i > 0 {
			ext.key = append(ext.key[:0], b[:i]...)
			b = b[i+1:]
			break
		}

		i = bytes.IndexByte(b, ';')
		if i == -1 {
			ext.key = append(ext.key[:0], b...)
			b = b[:0]
			break
		}
		ext.key = append(ext.key[:0], b[:i]...)
		if i == len(b) {
			b = b[:0]
			break
		}
		b = b[i+1:]
		ext.params = ext.params[:0]

		for {
			b = skipWhiteSpace(b)
			if len(b) == 0 {
				break
			}

			p := paramPool.Get().(*parameter)

			i = bytes.IndexByte(b, '=')
			if i == -1 {
				p.key = append(p.key[:0], b...)
				ext.params = append(ext.params, p)
				b = b[:0]
				break
			}

			p.key = append(p.key[:0], b[:i]...)
			if i == len(b) {
				break
			}

			b = b[i+1:]
			i = bytes.IndexByte(b, ';')
			if i == -1 {
				i = len(b)
			}

			p.value = append(p.value[:0], b[:i]...)
			ext.params = append(ext.params, p)
			if i == len(b) {
				b = b[:0]
				break
			}

			b = b[i+1:]
		}
	}
	return b
}

func parseExtensions(ctx *fasthttp.RequestCtx) []*extension {
	var exts []*extension
	ctx.Request.Header.VisitAll(func(k, v []byte) {
		if bytes.Equal(k, wsHeaderExtensions) {
			for len(v) > 0 {
				ext := extPool.Get().(*extension)
				v = ext.parse(v)
				exts = append(exts, ext)
			}
		}
	})
	return exts
}
