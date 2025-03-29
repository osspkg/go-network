/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal_test

import (
	"net"
	"testing"

	"go.osspkg.com/casecheck"
	"go.osspkg.com/ioutils/data"

	"go.osspkg.com/network/internal"
)

type mockConn struct {
	B *data.Buffer
}

func (m *mockConn) WriteTo(p []byte, _ net.Addr) (n int, err error) {
	return m.B.Write(p)
}

func TestUnit_PacketWrite(t *testing.T) {
	a := internal.PacketWrite{
		Addr: nil,
		Conn: &mockConn{
			B: data.NewBuffer(0),
		},
	}

	n, err := a.Write(make([]byte, 100_000))
	casecheck.NoError(t, err)
	casecheck.Equal(t, 100_000, n)
}
