/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package packet

import (
	"net"
	"slices"
)

const (
	Size = 65535
)

type Writer struct {
	Addr net.Addr
	Conn interface {
		WriteTo(p []byte, addr net.Addr) (n int, err error)
	}
}

func (a *Writer) Write(p []byte) (int, error) {
	var sum int
	for it := range slices.Chunk(p, Size) {
		n, err := a.Conn.WriteTo(it, a.Addr)
		sum += n

		if err != nil {
			return sum, err
		}
		if n == 0 {
			break
		}
	}

	return sum, nil
}
