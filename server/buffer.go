package server

import (
	"bytes"

	"go.osspkg.com/ioutils/pool"
)

var bytesPool = pool.New[*Bytes](func() *Bytes {
	return &Bytes{Slice: make([]byte, 65535)}
})

type Bytes struct {
	Slice []byte
}

func (*Bytes) Reset() {}

var bufferPool = pool.New[*bytes.Buffer](func() *bytes.Buffer {
	return bytes.NewBuffer(make([]byte, 0, 65535))
})
