package fastws

import (
	"bytes"
	"testing"
)

var (
	toUnmask = []byte{68, 5, 230, 92, 99, 64, 253, 95, 126, 12, 238, 16, 124, 20, 229, 95, 99}
	unmasked = []byte("Hello world ptooo")
)

func TestUnmask(t *testing.T) {
	key := []byte{12, 96, 138, 48, 255}
	mask(key, toUnmask)
	if !bytes.Equal(unmasked, toUnmask) {
		t.Fatalf("%s <> %s", toUnmask, unmasked)
	}
}
