package resp

import "io"

type RESPStream struct {
	r io.Reader
	c chan *RESP
}

func (r *RESPStream) Channel() <-chan *RESP {
	return r.c
}
