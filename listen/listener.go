/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package listen

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"

	"github.com/quic-go/quic-go"

	"go.osspkg.com/network/internal"
)

func New(ctx context.Context, network, address string, ssl *SSL) (io.Closer, error) {
	switch network {
	case internal.NetTCP:
		return newListen(ctx, network, address, ssl)
	case internal.NetUDP:
		return newListenPacket(ctx, network, address)
	case internal.NetUNIX:
		return newListen(ctx, network, address, nil)
	case internal.NetQUIC:
		return newListenQUIC(ctx, address, ssl)
	default:
		return nil, fmt.Errorf("invalid network type, use: tcp, udp, unix")
	}
}

func newListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	var lc net.ListenConfig
	return lc.ListenPacket(ctx, network, address)
}

func newListen(ctx context.Context, network, address string, ssl *SSL) (l net.Listener, err error) {
	var lc net.ListenConfig
	if l, err = lc.Listen(ctx, network, address); err != nil {
		return nil, err
	}

	if ssl == nil || len(ssl.Certs) == 0 {
		return
	}

	var conf *tls.Config
	if conf, err = NewTLSConfig(ssl); err != nil {
		return nil, err
	}
	return tls.NewListener(l, conf), nil
}

func newListenQUIC(_ context.Context, address string, ssl *SSL) (l *quic.Listener, err error) {
	if ssl == nil || len(ssl.Certs) == 0 {
		return nil, fmt.Errorf("QUIC cant work without tls")
	}

	if len(ssl.NextProtos) == 0 {
		ssl.NextProtos = append(ssl.NextProtos, "quic")
	}

	var conf *tls.Config
	if conf, err = NewTLSConfig(ssl); err != nil {
		return nil, err
	}

	return quic.ListenAddr(address, conf, &quic.Config{EnableDatagrams: true})
}
