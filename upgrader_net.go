package fastws

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/valyala/fasthttp"
)

type (
	// NetUpgradeHandler is the upgrading handler for net/http.
	NetUpgradeHandler func(resp http.ResponseWriter, req *http.Request) bool
)

// NetUpgrader upgrades HTTP connection to a websocket connection if it's possible.
//
// NetUpgrader executes NetUpgrader.Handler after successful websocket upgrading.
type NetUpgrader struct {
	// UpgradeHandler allows user to handle RequestCtx when upgrading.
	//
	// If UpgradeHandler returns false the connection won't be upgraded and
	// the parsed ctx will be used as a response.
	UpgradeHandler NetUpgradeHandler

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

// Upgrade upgrades HTTP to websocket connection if possible.
//
// If client does not request any websocket connection this function
// will execute ctx.NotFound()
//
// When connection is successfully stablished the function calls s.Handler.
func (upgr *NetUpgrader) Upgrade(resp http.ResponseWriter, req *http.Request) {
	rs := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(rs)

	if req.Method != "GET" {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	// Checking Origin header if needed
	origin := req.Header.Get("Origin")
	if upgr.Origin != "" {
		uri := fasthttp.AcquireURI()
		uri.Update(upgr.Origin)

		b := bytePool.Get().([]byte)
		b = prepareOrigin(b, uri)
		fasthttp.ReleaseURI(uri)

		if !equalsFold(b, s2b(origin)) {
			resp.WriteHeader(http.StatusForbidden)
			bytePool.Put(b)
			return
		}
		bytePool.Put(b)
	}

	// Normalizing must be disabled because of WebSocket header fields.
	// (This is not a fasthttp bug).
	rs.Header.DisableNormalizing()

	hasUpgrade := func() bool {
		for _, v := range req.Header["Connection"] {
			if strings.Contains(v, "Upgrade") {
				return true
			}
		}
		return false
	}()

	// Connection.Value == Upgrade
	if hasUpgrade {
		// Peek upgrade header field.
		hup := req.Header.Get("Upgrade")
		// Compare with websocket string defined by the RFC
		if equalsFold(s2b(hup), websocketString) {
			// Checking websocket version
			hversion := req.Header.Get(b2s(wsHeaderVersion))
			// Peeking websocket key.
			hkey := req.Header.Get(b2s(wsHeaderKey))
			hprotos := bytes.Split( // TODO: Reduce allocations. Do not split. Use IndexByte
				s2b(req.Header.Get(b2s(wsHeaderProtocol))), commaString,
			)
			supported := false
			// Checking versions
			for i := range supportedVersions {
				if bytes.Contains(supportedVersions[i], s2b(hversion)) {
					supported = true
					break
				}
			}
			if !supported {
				resp.WriteHeader(http.StatusBadRequest)
				io.WriteString(resp, "Versions not supported")
				return
			}

			if upgr.UpgradeHandler != nil {
				if !upgr.UpgradeHandler(resp, req) {
					return
				}
			}
			// TODO: compression
			//compress := mustCompress(exts)
			compress := false

			h, ok := resp.(http.Hijacker)
			if !ok {
				resp.WriteHeader(http.StatusInternalServerError)
				return
			}

			c, _, err := h.Hijack()
			if err != nil {
				io.WriteString(resp, err.Error())
				return
			}

			// Setting response headers
			rs.SetStatusCode(fasthttp.StatusSwitchingProtocols)
			rs.Header.AddBytesKV(connectionString, upgradeString)
			rs.Header.AddBytesKV(upgradeString, websocketString)
			rs.Header.AddBytesKV(wsHeaderAccept, makeKey(s2b(hkey), s2b(hkey)))
			// TODO: implement bad websocket version
			// https://tools.ietf.org/html/rfc6455#section-4.4
			if proto := selectProtocol(hprotos, upgr.Protocols); proto != "" {
				rs.Header.AddBytesK(wsHeaderProtocol, proto)
			}

			_, err = rs.WriteTo(c)
			if err != nil {
				c.Close()
				return
			}

			go func() {
				conn := acquireConn(c)
				// stablishing default options
				conn.server = true
				conn.compress = compress
				// executing handler
				upgr.Handler(conn)
				// closes and release the connection
				conn.Close()
				releaseConn(conn)
			}()
		}
	}
}
