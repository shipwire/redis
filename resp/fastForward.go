package resp

import "io"

func fastForwardReaderTail(r io.Reader, length, skipTail int64) io.Reader {
	ff := ffReader{
		raw:    r,
		r:      io.LimitReader(r, length),
		tail:   skipTail,
		length: length,
	}
	return ff
}

type ffReader struct {
	r, raw          io.Reader
	n, tail, length int64
}

func (f ffReader) Read(b []byte) (int, error) {
	n, err := f.r.Read(b)
	f.n += int64(n)
	if f.n >= f.length {
		f.raw.Read(make([]byte, f.tail))
	}
	return n, err
}
