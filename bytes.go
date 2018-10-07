package fastws

import "sync"

var bytePool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 128)
	},
}

func extendByteSlice(b []byte, needLen int) []byte {
	b = b[:cap(b)]
	if n := needLen - cap(b); n > 0 {
		b = append(b, make([]byte, n)...)
	}
	return b[:needLen]
}
