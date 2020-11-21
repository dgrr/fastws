package fastws

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net"

	"github.com/valyala/fasthttp"
)

var (
	// ErrCannotUpgrade shows up when an error ocurred when upgrading a connection.
	ErrCannotUpgrade = errors.New("cannot upgrade connection")
)

// Client returns Conn using existing connection.
//
// url must be complete URL format i.e. http://localhost:8080/ws
func Client(c net.Conn, url string) (*Conn, error) {
	return client(c, url, nil)
}

// ClientWithHeaders returns a Conn using existing connection and sending personalized headers.
func ClientWithHeaders(c net.Conn, url string, req *fasthttp.Request) (*Conn, error) {
	return client(c, url, req)
}

// UpgradeAsClient will upgrade the connection as a client
//
// r can be nil.
func UpgradeAsClient(c net.Conn, url string, r *fasthttp.Request) error {
	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)
	defer fasthttp.ReleaseURI(uri)

	uri.Update(url)

	origin := bytePool.Get().([]byte)
	key := bytePool.Get().([]byte)
	defer bytePool.Put(origin)
	defer bytePool.Put(key)

	origin = prepareOrigin(origin, uri)
	key = makeRandKey(key[:0])

	if r != nil {
		r.CopyTo(req)
	}

	req.Header.SetMethod("GET")
	req.Header.AddBytesKV(originString, origin)
	req.Header.AddBytesKV(connectionString, upgradeString)
	req.Header.AddBytesKV(upgradeString, websocketString)
	req.Header.AddBytesKV(wsHeaderVersion, supportedVersions[0])
	req.Header.AddBytesKV(wsHeaderKey, key)
	// TODO: Add compression

	req.SetRequestURIBytes(uri.FullURI())

	br := bufio.NewReader(c)
	bw := bufio.NewWriter(c)
	req.Write(bw)
	bw.Flush()
	err := res.Read(br)
	if err == nil {
		if res.StatusCode() != 101 ||
			!equalsFold(res.Header.PeekBytes(upgradeString), websocketString) {
			err = ErrCannotUpgrade
		}
	}

	return err
}

func client(c net.Conn, url string, r *fasthttp.Request) (conn *Conn, err error) {
	err = UpgradeAsClient(c, url, r)
	if err == nil {
		conn = acquireConn(c)
		conn.server = false
	}

	return conn, err
}

// Dial establishes a websocket connection as client.
//
// url parameter must follow WebSocket URL format i.e. ws://host:port/path
func Dial(url string) (*Conn, error) {
	cnf := &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS11,
	}
	return dial(url, cnf, nil)
}

// DialTLS establishes a websocket connection as client with the
// parsed tls.Config. The config will be used if the URL is wss:// like.
func DialTLS(url string, cnf *tls.Config) (*Conn, error) {
	return dial(url, cnf, nil)
}

// DialWithHeaders establishes a websocket connection as client sending a personalized request.
func DialWithHeaders(url string, req *fasthttp.Request) (*Conn, error) {
	cnf := &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS11,
	}
	return dial(url, cnf, req)
}

func dial(url string, cnf *tls.Config, req *fasthttp.Request) (conn *Conn, err error) {
	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	uri.Update(url)

	scheme := "https"
	port := ":443"
	if bytes.Equal(uri.Scheme(), wsString) {
		scheme, port = "http", ":80"
	}
	uri.SetScheme(scheme)

	addr := bytePool.Get().([]byte)
	defer bytePool.Put(addr)

	addr = append(addr[:0], uri.Host()...)
	if n := bytes.LastIndexByte(addr, ':'); n == -1 {
		addr = append(addr, port...)
	}

	var c net.Conn

	if scheme == "http" {
		c, err = net.Dial("tcp", b2s(addr))
	} else {
		c, err = tls.Dial("tcp", b2s(addr), cnf)
	}
	if err == nil {
		conn, err = client(c, uri.String(), req)
		if err != nil {
			c.Close()
		}
	}
	return conn, err
}

func makeRandKey(b []byte) []byte {
	b = extendByteSlice(b, 16)
	rand.Read(b[:16])
	b = appendEncode(base64, b[:0], b[:16])
	return b
}
