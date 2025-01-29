/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"context"
	"net"

	"go.osspkg.com/ioutils/data"
	"go.osspkg.com/ioutils/pool"
)

var (
	poolPRC = pool.New[*prc](newPRC)
)

type (
	tPRConn interface {
		WriteTo(p []byte, addr net.Addr) (n int, err error)
	}

	prc struct {
		conn tPRConn
		rb   *data.Buffer
		wb   *data.Buffer
		ctx  context.Context
		addr net.Addr
	}
)

func newPRC() *prc {
	return &prc{
		rb: data.NewBuffer(512),
		wb: data.NewBuffer(512),
	}
}

func (v *prc) Setup(ctx context.Context, conn tPRConn, addr net.Addr) {
	v.ctx = ctx
	v.conn = conn
	v.addr = addr
}

func (v *prc) Reset() {
	v.conn = nil
	v.rb.Reset()
	v.wb.Reset()
	v.ctx = nil
	v.addr = nil
}

func (v *prc) Pickup(b []byte) (int, error) {
	return v.rb.Write(b)
}

func (v *prc) Release() (int, error) {
	return v.conn.WriteTo(v.wb.Bytes(), v.addr)
}

func (v *prc) Read(b []byte) (int, error) {
	return v.rb.Read(b)
}

func (v *prc) Write(b []byte) (int, error) {
	return v.wb.Write(b)
}

func (v *prc) Addr() string {
	return v.addr.String()
}

func (v *prc) Context() context.Context {
	return v.ctx
}
