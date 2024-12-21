/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"bytes"
	"context"
	"io"
	"net"

	"go.osspkg.com/ioutils"
	"go.osspkg.com/ioutils/pool"
)

var (
	poolRWC = pool.New[*rwc](newRWC)
)

type (
	tRWConn interface {
		io.ReadWriter
	}

	rwc struct {
		conn  tRWConn
		rb    *bytes.Buffer
		wb    *bytes.Buffer
		bsize int
		ctx   context.Context
		addr  net.Addr
	}
)

func newRWC() *rwc {
	return &rwc{
		rb: bytes.NewBuffer(make([]byte, 0, 512)),
		wb: bytes.NewBuffer(make([]byte, 0, 512)),
	}
}

func (v *rwc) Setup(ctx context.Context, bsize int, conn tRWConn, addr net.Addr) {
	v.ctx = ctx
	v.conn = conn
	v.addr = addr
	v.bsize = bsize
}

func (v *rwc) Reset() {
	v.conn = nil
	v.rb.Reset()
	v.wb.Reset()
	v.bsize = 0
	v.ctx = nil
	v.addr = nil
}

func (v *rwc) Pickup() error {
	n, err := ioutils.CopyPack(v.rb, v.conn, v.bsize)
	if err != nil {
		return err
	}
	if n == 0 {
		return io.EOF
	}
	return nil
}

func (v *rwc) Release() error {
	n, err := ioutils.CopyPack(v.conn, v.wb, v.bsize)
	if err != nil {
		return err
	}
	if n == 0 {
		return io.EOF
	}
	return nil
}

func (v *rwc) Read(b []byte) (int, error) {
	return v.rb.Read(b)
}

func (v *rwc) Write(b []byte) (int, error) {
	return v.wb.Write(b)
}

func (v *rwc) Addr() string {
	return v.addr.String()
}

func (v *rwc) Context() context.Context {
	return v.ctx
}
