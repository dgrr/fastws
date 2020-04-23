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

type (
	// RequestHandler is the websocket connection handler.
	RequestHandler func(conn *Conn)
	// UpgradeHandler is the upgrading handler.
	UpgradeHandler func(*fasthttp.RequestCtx) bool
)

// Upgrader upgrades HTTP connection to a websocket connection if it's possible.
//
// Upgrader executes Upgrader.Handler after successful websocket upgrading.
type Upgrader struct {
	// UpgradeHandler allows user to handle RequestCtx when upgrading.
	//
	// If UpgradeHandler returns false the connection won't be upgraded and
	// the parsed ctx will be used as a response.
	UpgradeHandler UpgradeHandler

	// Handler is the request handler for ws connections.
	Handler RequestHandler

	// Protocols are the supported protocols.
	Protocols []string

	// Origin is used to limit the clients coming from the defined origin
	Origin string

	// Compress defines whether using compression or not.
	// TODO
	Compress bool
}

func prepareOrigin(b []byte, uri *fasthttp.URI) []byte {
	b = append(b[:0], uri.Scheme()...)
	b = append(b, "://"...)
	return append(b, uri.Host()...)
}

// Upgrade upgrades HTTP to websocket connection if possible.
//
// If client does not request any websocket connection this function
// will execute ctx.NotFound()
//
// When connection is successfully stablished the function calls s.Handler.
func (upgr *Upgrader) Upgrade(ctx *fasthttp.RequestCtx) {
	if !ctx.IsGet() {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	// Checking Origin header if needed
	origin := ctx.Request.Header.Peek("Origin")
	if upgr.Origin != "" {
		uri := fasthttp.AcquireURI()
		uri.Update(upgr.Origin)

		b := bytePool.Get().([]byte)
		b = prepareOrigin(b, uri)
		fasthttp.ReleaseURI(uri)

		if !equalsFold(b, origin) {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			bytePool.Put(b)
			return
		}
		bytePool.Put(b)
	}

	// Normalizing must be disabled because of WebSocket header fields.
	// (This is not a fasthttp bug).
	ctx.Response.Header.DisableNormalizing()

	// Connection.Value == Upgrade
	if ctx.Request.Header.ConnectionUpgrade() {
		// Peek upgrade header field.
		hup := ctx.Request.Header.PeekBytes(upgradeString)
		// Compare with websocket string defined by the RFC
		if equalsFold(hup, websocketString) {
			// Checking websocket version
			hversion := ctx.Request.Header.PeekBytes(wsHeaderVersion)
			// Peeking websocket key.
			hkey := ctx.Request.Header.PeekBytes(wsHeaderKey)
			hprotos := bytes.Split( // TODO: Reduce allocations. Do not split. Use IndexByte
				ctx.Request.Header.PeekBytes(wsHeaderProtocol), commaString,
			)
			supported := false
			// Checking versions
			for i := range supportedVersions {
				if bytes.Contains(supportedVersions[i], hversion) {
					supported = true
					break
				}
			}
			if !supported {
				ctx.Error("Versions not supported", fasthttp.StatusBadRequest)
				return
			}

			if upgr.UpgradeHandler != nil {
				if !upgr.UpgradeHandler(ctx) {
					return
				}
			}
			// TODO: compression
			//compress := mustCompress(exts)
			compress := false

			// Setting response headers
			ctx.Response.SetStatusCode(fasthttp.StatusSwitchingProtocols)
			ctx.Response.Header.AddBytesKV(connectionString, upgradeString)
			ctx.Response.Header.AddBytesKV(upgradeString, websocketString)
			ctx.Response.Header.AddBytesKV(wsHeaderAccept, makeKey(hkey, hkey))
			// TODO: implement bad websocket version
			// https://tools.ietf.org/html/rfc6455#section-4.4
			if proto := selectProtocol(hprotos, upgr.Protocols); proto != "" {
				ctx.Response.Header.AddBytesK(wsHeaderProtocol, proto)
			}

			userValues := make(map[string]interface{})
			ctx.VisitUserValues(func(k []byte, v interface{}) {
				userValues[string(k)] = v
			})

			ctx.Hijack(func(c net.Conn) {
				conn := acquireConn(c)
				// stablishing default options
				conn.server = true
				conn.compress = compress
				conn.userValues = userValues

				// executing handler
				upgr.Handler(conn)

				// closes and release the connection
				conn.Close()
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

func makeKey(dst, key []byte) []byte {
	h := shaPool.Get().(hash.Hash)
	h.Reset()
	defer shaPool.Put(h)

	h.Write(key)
	h.Write(uidKey)
	dst = h.Sum(dst[:0])
	dst = appendEncode(base64, dst, dst)
	return dst
}

// Thank you @valyala
//
// https://go-review.googlesource.com/c/go/+/37639
func appendEncode(enc *b64.Encoding, dst, src []byte) []byte {
	n := enc.EncodedLen(len(src)) + len(dst)
	b := extendByteSlice(dst, n)
	n = len(dst)
	enc.Encode(b[n:], src)
	return b[n:]
}

func appendDecode(enc *b64.Encoding, dst, src []byte) ([]byte, error) {
	needLen := enc.DecodedLen(len(src)) + len(dst)
	b := extendByteSlice(dst, needLen)
	n, err := enc.Decode(b[len(dst):], src)
	return b[:len(dst)+n], err
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
