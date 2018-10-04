package fastws

import (
	"bufio"
	"bytes"
	"errors"
	"net"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastrand"
)

var (
	ErrCannotUpgrade = errors.New("cannot upgrade connection status != 101")
)

// Dial performs establishes websocket connection as client.
//
// url parameter must follow WebSocket url format i.e. ws://host:port/path
func Dial(url string) (conn *Conn, err error) {
	uri := fasthttp.AcquireURI()
	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseURI(uri)
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	uri.Update(url)

	scheme := "https"
	if bytes.Equal(uri.Scheme(), wsString) {
		scheme = "http"
	}
	uri.SetScheme(scheme)

	req.Header.SetMethod("GET")
	req.Header.AddBytesKV(connectionString, upgradeString)
	req.Header.AddBytesKV(upgradeString, websocketString)
	req.Header.AddBytesKV(wsHeaderVersion, supportedVersions[0])
	req.Header.AddBytesKV(wsHeaderKey, makeRandKey(nil))
	// TODO: Add compression
	req.SetRequestURIBytes(uri.FullURI())

	var c net.Conn
	c, err = net.Dial("tcp4", b2s(uri.Host()))
	if err == nil {
		bw := bufio.NewWriter(c)
		br := bufio.NewReader(c)
		req.Write(bw)
		bw.Flush()
		res.Read(br)

		if res.StatusCode() != 101 &&
			bytes.Equal(res.Header.PeekBytes(upgradeString), websocketString) {
			c.Close()
			err = ErrCannotUpgrade
		} else {
			conn = acquireConn(c)
			conn.server = false
		}
	}
	return conn, err
}

var randLetters = []byte("qwtuiopasdgjklzxcbnmWQETUIOASDFGHJKXCVBNM")

func makeRandKey(b []byte) []byte {
	b = b[:0]
	n := uint32(len(randLetters))
	for i := 0; i < 16; i++ {
		b = append(b, randLetters[fastrand.Uint32n(n)])
	}
	return s2b(base64.EncodeToString(b))
}
