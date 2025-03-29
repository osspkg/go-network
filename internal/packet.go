/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import (
	"net"
)

const (
	UDPPacketSize = 65535
)

type PacketWrite struct {
	Addr net.Addr
	Conn interface {
		WriteTo(p []byte, addr net.Addr) (n int, err error)
	}
}

func (a *PacketWrite) Write(p []byte) (n int, err error) {
	from, count := 0, len(p)

	defer func() {
		n = from
	}()

	for i := 0; i < count; i += UDPPacketSize {
		to := min(count, from+UDPPacketSize)
		if n, err = a.Conn.WriteTo(p[from:to], a.Addr); err != nil {
			break
		}
		from += n
	}

	return
}
