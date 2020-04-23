package fastws

import (
	"bytes"
	"testing"
)

var (
	bstr = []byte("This string must be equals")
	cstr = []byte("This StrING Must bE equAls")
)

func TestEqualsFold(t *testing.T) {
	if !equalsFold(bstr, cstr) {
		t.Fatal("equalsFold error")
	}
}

func BenchmarkMineEqualFold(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if !equalsFold(bstr, cstr) {
			b.Fatal("error checking equality")
		}
	}
}

func BenchmarkBytesEqualFold(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if !bytes.EqualFold(bstr, cstr) {
			b.Fatal("error checking equality")
		}
	}
}
