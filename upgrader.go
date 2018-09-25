package fastws

import (
	"bytes"
	"crypto/sha1"
	b64 "encoding/base64"
	"hash"
	"net"
	"sync"

	"github.com/valyala/fasthttp"
)

// RequestHandler is the websocket handler.
type RequestHandler func(conn *Conn)

// Upgrader upgrades HTTP connection to a websocket connection if it's possible.
//
// Upgrader executes Upgrader.Handler after successful websocket upgrading.
type Upgrader struct {
	// Handler is the request handler for ws connections.
	Handler RequestHandler

	// Protocols are the supported protocols.
	Protocols []string

	// Origin ...
	Origin string

	// Compress ...
	Compress bool
}

// Upgrader upgrades HTTP to websocket connection if possible.
//
// If client does not request any websocket connection this function
// will execute ctx.NotFound()
//
// When connection is successfully stablished this function calls s.Handler.
func (upgr *Upgrader) Upgrade(ctx *fasthttp.RequestCtx) {
	if !ctx.IsGet() {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	origin := b2s(ctx.Request.Header.Peek("Origin"))
	if upgr.Origin != "" && upgr.Origin != origin {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		return
	}

	hconn := ctx.Request.Header.PeekBytes(connectionString)
	if bytes.Equal(hconn, upgradeString) {
		hup := ctx.Request.Header.PeekBytes(upgradeString)
		if bytes.Equal(hup, wsString) {
			hversion := ctx.Request.Header.PeekBytes(wsHeaderVersion)
			hkey := ctx.Request.Header.PeekBytes(wsHeaderKey)
			hprotos := bytes.Split(
				ctx.Request.Header.PeekBytes(wsHeaderProtocol), commaString,
			)
			supported := false
			for i := range supportedVersions {
				if bytes.Equal(supportedVersions[i], hversion) {
					supported = true
					break
				}
			}
			if !supported {
				ctx.Error("Not supported version", fasthttp.StatusBadRequest)
				return
			}
			exts := parseExtensions(ctx)
			compress := mustCompress(exts)

			ctx.Response.SetStatusCode(fasthttp.StatusSwitchingProtocols)
			ctx.Response.Header.AddBytesKV(connectionString, upgradeString)
			ctx.Response.Header.AddBytesKV(upgradeString, wsString)
			ctx.Response.Header.AddBytesK(wsHeaderAccept, makeKey(hkey, hkey))
			// TODO: implement bad websocket version
			// https://tools.ietf.org/html/rfc6455#section-4.4
			if proto := selectProtocol(hprotos, upgr.Protocols); proto != "" {
				ctx.Response.Header.AddBytesK(wsHeaderProtocol, proto)
			}

			ctx.Hijack(func(c net.Conn) {
				conn := acquireConn(c)
				conn.exts = append(conn.exts[:0], exts...)
				// release extensions
				releaseExtensions(exts)
				exts = nil
				// stablishing default options
				conn.server = true
				conn.compress = compress
				// executing handler
				upgr.Handler(conn)
				// closes and release the connection
				conn.Close("")
				releaseConn(conn)
			})
		}
	}
}

var shaPool = sync.Pool{
	New: func() interface{} {
		return sha1.New()
	},
}

var base64 = b64.StdEncoding

func makeKey(dst, key []byte) string {
	h := shaPool.Get().(hash.Hash)
	h.Reset()
	defer shaPool.Put(h)

	h.Write(key)
	h.Write(uidKey)
	dst = h.Sum(dst[:0])
	// TODO: Avoid extra allocations
	return base64.EncodeToString(dst)
}

func selectProtocol(protos [][]byte, accepted []string) string {
	if len(protos) == 0 {
		return ""
	}

	for _, proto := range protos {
		for _, accept := range accepted {
			if b2s(proto) == accept {
				return accept
			}
		}
	}
	return string(protos[0])
}

func mustCompress(exts []*extension) bool {
	c := false
	for _, ext := range exts {
		if bytes.Equal(ext.key, permessageDeflate) {
			c = true
			break
		}
	}
	return c
}
