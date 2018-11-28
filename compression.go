package fastws

import (
	"bytes"
	"compress/flate"
	"io"
	"sync"

	//"github.com/klauspost/compress/flate"
	"github.com/valyala/fasthttp"
)

var (
	flateReaderPool sync.Pool
	flateWriterPool sync.Pool
)

func mustCompress(req *fasthttp.Request) (must bool) {
	exts := req.Header.PeekBytes(wsHeaderExtensions)
	i := bytes.IndexByte(exts, ',')
	if i == -1 {
		must = bytes.Contains(exts, permessageDeflate)
	} else {
		for i > 0 {
			must = bytes.Contains(exts[:i], permessageDeflate)
			if must {
				break
			}
			exts = exts[i+1:]
			i = bytes.IndexByte(exts, ',')
		}
	}
	return
}

func resetFlateReader(fr io.ReadCloser, r io.Reader) error {
	frr, ok := fr.(flate.Resetter)
	if !ok {
		panic("BUG: flate.Reader doesn't implement flate.Resetter???")
	}
	return frr.Reset(r, nil)
}

func acquireFlateWriter(w io.Writer) (fw io.WriteCloser, err error) {
	v := flateWriterPool.Get()
	if v == nil {
		fw, err = flate.NewWriter(w, flate.BestCompression) // TODO: Change mode?
	} else {
		fw = v.(io.WriteCloser)
		fw.(*flate.Writer).Reset(w) // TODO: review
	}
	return
}

func acquireFlateReader(r io.Reader) (fr io.ReadCloser, err error) {
	v := flateReaderPool.Get()
	if v == nil {
		fr = flate.NewReader(r)
	} else {
		fr = v.(io.ReadCloser)
		err = resetFlateReader(fr, r)
	}
	return
}

func releaseFlateReader(fr io.ReadCloser) {
	fr.Close()
	flateReaderPool.Put(fr)
}

func releaseFlateWriter(fw io.WriteCloser) {
	fw.Close()
	flateWriterPool.Put(fw)
}
