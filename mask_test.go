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
	m := make([]byte, len(toUnmask))
	key := []byte{12, 96, 138, 48, 255}
	copy(m, toUnmask)
	mask(key, m)
	if !bytes.Equal(unmasked, m) {
		t.Fatalf("%v <> %s", m, unmasked)
	}
}
