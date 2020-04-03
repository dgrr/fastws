package fastws

import (
	"bufio"
	"bytes"
	"io"
	"testing"
)

var (
	littlePacket = []byte{0x81, 0x05, 72, 101, 108, 108, 111}
	hugePacket   = []byte{0x81, 126, 0, 252, 182, 94, 215, 80, 169, 88, 21, 155, 191, 28, 138, 68, 178, 44, 170, 219, 130, 133, 85, 166, 58, 147, 227, 86, 70, 176, 19, 210, 89, 220, 34, 155, 41, 22, 253, 72, 204, 255, 114, 37, 226, 180, 45, 102, 239, 178, 111, 33, 88, 192, 211, 248, 146, 132, 97, 230, 107, 155, 60, 230, 156, 114, 247, 95, 247, 124, 57, 5, 172, 104, 140, 156, 245, 201, 126, 165, 106, 4, 92, 168, 20, 90, 50, 179, 33, 126, 165, 195, 225, 43, 65, 177, 17, 165, 30, 147, 38, 24, 65, 148, 44, 241, 53, 200, 182, 137, 232, 56, 201, 227, 189, 215, 16, 110, 211, 219, 221, 103, 154, 165, 246, 186, 27, 28, 80, 82, 39, 201, 58, 11, 249, 194, 59, 230, 69, 166, 185, 62, 3, 15, 249, 56, 175, 174, 200, 33, 104, 106, 127, 222, 84, 140, 85, 209, 138, 181, 90, 168, 0, 250, 15, 75, 10, 3, 30, 161, 153, 150, 60, 7, 56, 218, 140, 189, 121, 23, 102, 40, 183, 242, 84, 37, 166, 219, 117, 99, 219, 0, 88, 228, 55, 113, 112, 158, 26, 26, 107, 97, 247, 138, 7, 19, 86, 138, 17, 4, 168, 44, 120, 19, 89, 179, 237, 167, 198, 71, 208, 154, 12, 149, 236, 90, 37, 39, 111, 180, 173, 11, 30, 209, 93, 226, 148, 122, 198, 26, 97, 60, 61, 190, 227, 151, 60, 2, 119, 174, 123, 76, 107, 253, 78, 61}
)

func TestReadBufio(t *testing.T) {
	reader := bufio.NewReader(
		bytes.NewBuffer(littlePacket),
	)
	fr := AcquireFrame()

	_, err := fr.ReadFrom(reader)
	if err != nil {
		t.Fatal(err)
	}
	checkValues(fr, t, false, true, littlePacket[2:])

	ReleaseFrame(fr)
}

func TestReadHugeBufio(t *testing.T) {
	reader := bufio.NewReader(
		bytes.NewBuffer(hugePacket),
	)
	fr := AcquireFrame()

	_, err := fr.ReadFrom(reader)
	if err != nil {
		t.Fatal(err)
	}
	checkValues(fr, t, false, true, hugePacket[4:])

	ReleaseFrame(fr)
}

func TestReadStd(t *testing.T) {
	fr := AcquireFrame()
	_, err := fr.ReadFrom(
		bytes.NewBuffer(littlePacket),
	)
	if err != nil {
		t.Fatal(err)
	}
	checkValues(fr, t, false, true, littlePacket[2:])
	ReleaseFrame(fr)
}

func TestReadHugeStd(t *testing.T) {
	fr := AcquireFrame()
	_, err := fr.ReadFrom(
		bytes.NewBuffer(hugePacket),
	)
	if err != nil {
		t.Fatal(err)
	}
	checkValues(fr, t, false, true, hugePacket[4:])
	ReleaseFrame(fr)
}

func checkValues(fr *Frame, t *testing.T, c, fin bool, payload []byte) {
	if fin && !fr.IsFin() {
		t.Fatal("Is not fin")
	}

	if c && !fr.IsContinuation() {
		t.Fatal("Is not continuation")
	}

	if int(fr.Len()) != len(payload) {
		t.Fatalf("Incorrect length: %d<>%d", fr.Len(), len(payload))
	}

	p := fr.Payload()
	if !bytes.Equal(p, payload) {
		t.Fatalf("Bad payload %s<>%s", p, payload)
	}
}

func BenchmarkRead(b *testing.B) {
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		var err error
		var r = bytes.NewBuffer(littlePacket)
		reader := bufio.NewReader(r)

		fr := AcquireFrame()
		for pb.Next() {
			_, err = fr.ReadFrom(reader)
			if err != nil && err != io.EOF {
				break
			}
			fr.Reset()
			reader.Reset(r)
			err = nil
		}
		if err != nil {
			b.Fatal(err)
		}
		ReleaseFrame(fr)
	})
}
