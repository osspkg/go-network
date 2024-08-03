/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package listen

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"net"

	netaddres "go.osspkg.com/network/address"
)

type Cert struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

func New(ctx context.Context, network, address string, certs ...Cert) (io.Closer, error) {
	address = netaddres.CheckHostPort(address)

	switch network {
	case "tcp":
		return newTCPListen(ctx, network, address, certs...)
	case "unix":
		return newTCPListen(ctx, network, address)
	case "udp":
		return newUDPListen(ctx, network, address)
	default:
		return nil, fmt.Errorf("invalid network type, use: tcp, udp, unix")
	}
}

func newUDPListen(ctx context.Context, network, address string) (net.PacketConn, error) {
	var lc net.ListenConfig
	l, err := lc.ListenPacket(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func newTCPListen(ctx context.Context, network, address string, certs ...Cert) (net.Listener, error) {
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return l, nil
	}
	config, err := tlsConfig(certs...)
	if err != nil {
		return nil, err
	}
	tl := tls.NewListener(l, config)
	return tl, nil
}

func tlsConfig(certs ...Cert) (*tls.Config, error) {
	certificates := make([]tls.Certificate, 0, len(certs))
	for _, c := range certs {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, err
		}
		certificates = append(certificates, cert)
	}
	config := tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: certificates,
		Rand:         rand.Reader,
		CurvePreferences: []tls.CurveID{
			tls.CurveP521,
			tls.CurveP384,
			tls.CurveP256,
		},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}
	return &config, nil
}
