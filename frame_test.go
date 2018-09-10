package fastws

import (
	"bufio"
	"bytes"
	"testing"
)

var (
	header  = []byte{0x81, 0x05}
	payload = []byte{72, 101, 108, 108, 111}
)

func getBuffer() *bytes.Buffer {
	return bytes.NewBuffer(append(header, payload...))
}

func TestReadBufio(t *testing.T) {
	reader := bufio.NewReader(getBuffer())
	fr := AcquireFrame()

	fr.ReadFrom(reader)
	checkValues(fr, t)

	ReleaseFrame(fr)
}

func TestReadStd(t *testing.T) {
	fr := AcquireFrame()
	fr.ReadFrom(getBuffer())
	checkValues(fr, t)
	ReleaseFrame(fr)
}

func checkValues(fr *Frame, t *testing.T) {
	if !fr.IsFin() {
		t.Fatal("Is not fin")
	}
	if int(fr.Len()) != len(payload) {
		t.Fatalf("Incorrect length: %d<>%d", fr.Len(), len(payload))
	}

	p := fr.Payload()
	if !bytes.Equal(p, payload) {
		t.Fatalf("Bad payload %s<>%s", p, payload)
	}
}
