/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import (
	"bytes"
	"fmt"
	"io"
	"net"

	"go.osspkg.com/errors"
	"go.osspkg.com/ioutils/pool"
)

const buffSize = 512

var (
	BuffPool = pool.New[*bytes.Buffer](func() *bytes.Buffer {
		return bytes.NewBuffer(make([]byte, 0, buffSize))
	})

	BytesPool = pool.New[*Bytes](func() *Bytes {
		return &Bytes{Slice: make([]byte, buffSize)}
	})
)

type Bytes struct {
	Slice []byte
}

func (*Bytes) Reset() {}

// ====================================================================================================================

type ReadFrom interface {
	ReadFrom(p []byte) (n int, addr net.Addr, err error)
}

func CopyFrom(w io.Writer, r ReadFrom) (n int, addr net.Addr, err error) {
	var m int
	buff := BytesPool.Get()
	defer BytesPool.Put(buff)
	for {
		m, addr, err = r.ReadFrom(buff.Slice)
		if m < 0 {
			err = fmt.Errorf("reader err: negative read bytes")
			return
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return
		}
		n += m
		_, err2 := w.Write(buff.Slice[:m])
		if err2 != nil {
			err = fmt.Errorf("writer err: %w", err2)
			return
		}
		if m < buffSize || errors.Is(err, io.EOF) {
			err = nil
			return
		}
	}
}

type WriteTo interface {
	WriteTo(p []byte, addr net.Addr) (n int, err error)
}

func CopyTo(w WriteTo, r io.Reader, addr net.Addr) (n int, err error) {
	var m int
	buff := BytesPool.Get()
	defer BytesPool.Put(buff)
	for {
		m, err = r.Read(buff.Slice)
		if m < 0 {
			err = fmt.Errorf("reader err: negative read bytes")
			return
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return
		}
		n += m
		_, err2 := w.WriteTo(buff.Slice[:m], addr)
		if err2 != nil {
			err = fmt.Errorf("writer err: %w", err2)
			return
		}
		if m < buffSize || errors.Is(err, io.EOF) {
			err = nil
			return
		}
	}
}
