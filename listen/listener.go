/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package listen

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"

	"go.osspkg.com/network/xnet"
)

func New(ctx context.Context, network, address string, ssl *tls.Config) (io.Closer, error) {
	switch network {
	case xnet.NetTCP:
		return newListen(ctx, network, address, ssl)
	case xnet.NetUDP:
		return newListenPacket(ctx, network, address)
	case xnet.NetUNIX:
		return newListen(ctx, network, address, nil)
	default:
		return nil, fmt.Errorf("invalid network type, use: tcp, udp, unix")
	}
}

func newListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	var lc net.ListenConfig
	return lc.ListenPacket(ctx, network, address)
}

func newListen(ctx context.Context, network, address string, ssl *tls.Config) (l net.Listener, err error) {
	var lc net.ListenConfig
	if l, err = lc.Listen(ctx, network, address); err != nil {
		return nil, err
	}

	if ssl == nil {
		return
	}

	return tls.NewListener(l, ssl), nil
}
