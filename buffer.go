package fastws

type bufferWriter struct {
	wr io.Writer
	b  []byte
	n  int
}

func (bw *bufferWriter) Write(b []byte) (int, error) {
	bw.b = append(bw.b[:bw.n], b...)
	n := len(b)
	bw.b += n
	return n
}

func (bw *bufferWriter) Flush() error {
	n, err := bw.wr.Write(bw.b[:bw.n])
	bw.n -= n
	return err
}

func (bw *bufferWriter) Reset(wr io.Writer) {
	bw.wr = wr
	bw.b = bw.b[:0]
	bw.n = 0
}
