/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package epoll

import (
	"net"
)

type (
	connect struct {
		conn net.Conn
		fd   int32
	}

	TConnect interface {
		FD() int32
		Conn() net.Conn
	}
)

func newConnect(c net.Conn, fd int32) TConnect {
	return &connect{
		conn: c,
		fd:   fd,
	}
}

func (v *connect) Conn() net.Conn {
	return v.conn
}

func (v *connect) FD() int32 {
	return v.fd
}
