package fastws

import (
	"github.com/valyala/fasthttp"
)

// Dialer ...
type Dailer struct {
	Compress bool

	Client fasthttp.Client
}

func (dialer *Dialer) Dial(url string) (*Conn, error) {
	req, res := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)
	defer fasthttp.ReleaseURI(uri)

	uri.Update(url)

	req.SetRequestURI(url)
	req.Header.AddKV(originString, buildUri(uri))
	req.Header.AddKV(connectionString, upgradeString)
	req.Header.AddKV(connectionString, wsString)
	req.Header.AddKV(wsHeaderVersion, supportedVersions[0])
	req.Header.AddKV(wsHeaderKey, buildKey())
	// TODO: Add support for protocols and extensions

	err := dialer.Client.Do(req, res)
	if err != nil {
		return nil, err
	}
	if res.StatusCode() != 101 {
		return nil, fmt.Errorf("Unexpected status code %d", res.StatusCode())
	}
	// TODO: Check values
}

func buildUri(uri *fasthttp.URI) []byte {
	return append(
		append(uri.Scheme(), ':', '/', '/'), uri.Host()...,
	)
}
