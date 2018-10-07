package fastws

import (
	"bytes"
	"testing"
)

var (
	decodedString = []byte("hello world")
	encodedString = []byte("aGVsbG8gd29ybGQ=")
)

func TestBase64Encoding(t *testing.T) {
	var b = make([]byte, len(decodedString))
	copy(b, decodedString)
	b = appendEncode(base64, b[:0], b)
	if !bytes.Equal(b, encodedString) {
		t.Fatalf("bad encoding: %s <> %s", b, encodedString)
	}
}

func TestBase64Decoding(t *testing.T) {
	var b = make([]byte, len(encodedString))
	var err error
	copy(b, encodedString)
	b, err = appendDecode(base64, b[:0], b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, decodedString) {
		t.Fatalf("bad decoding: %s <> %s", b, decodedString)
	}
}

func BenchmarkBase64Encoding(b *testing.B) {
	var bf []byte
	for i := 0; i < b.N; i++ {
		bf = appendEncode(base64, bf[:0], decodedString)
		if !bytes.Equal(bf, encodedString) {
			b.Fatalf("%s <> %s", bf, encodedString)
		}
	}
}

func BenchmarkBase64Decoding(b *testing.B) {
	var bf []byte
	var err error
	for i := 0; i < b.N; i++ {
		bf, err = appendDecode(base64, bf[:0], encodedString)
		if err != nil {
			b.Fatal(err)
		}
		if !bytes.Equal(bf, decodedString) {
			b.Fatalf("%s <> %s", bf, decodedString)
		}
	}
}
