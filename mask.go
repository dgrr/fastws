package fastws

import (
	"crypto/rand"
)

func mask(mask, b []byte) {
	for i := range b {
		b[i] ^= mask[i&3]
	}
}

func readMask(b []byte) {
	rand.Read(b)
}
