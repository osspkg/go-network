/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"fmt"
	"net"

	"go.osspkg.com/network/internal"
)

type Config struct {
	Network     string
	Address     string
	Certificate *Certificate
	MaxConns    uint64
}

func (c Config) Resolve() (addr fmt.Stringer, err error) {
	if err := internal.IsPassableNetwork(c.Network); err != nil {
		return nil, err
	}

	switch c.Network {
	case internal.NetTCP:
		return net.ResolveTCPAddr(internal.NetTCP, c.Address)
	case internal.NetUDP, internal.NetQUIC:
		return net.ResolveUDPAddr(internal.NetUDP, c.Address)
	case internal.NetUNIX:
		return net.ResolveUnixAddr(internal.NetUNIX, c.Address)
	default:
		return nil, fmt.Errorf("invalid network name, use: tcp, udp, unix, quic")
	}
}
