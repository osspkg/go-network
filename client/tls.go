/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"

	"go.osspkg.com/network/internal"
)

type Certificate struct {
	CAFile             string `yaml:"ca_file"`
	CertFile           string `yaml:"cert_file"`
	KeyFile            string `yaml:"key_file"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

func (c *Certificate) parse() (cert tls.Certificate, ca *x509.CertPool, err error) {
	if len(c.CertFile) > 0 || len(c.KeyFile) > 0 {
		cert, err = tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return
		}
	}
	if len(c.CAFile) > 0 {
		ca = x509.NewCertPool()
		if caCert, e := os.ReadFile(c.CAFile); e == nil {
			ca.AppendCertsFromPEM(caCert)
		}
	}
	return
}

func (c *Certificate) Config(address, network string) (*tls.Config, error) {
	if c == nil {
		return nil, nil
	}

	switch network {
	case internal.NetUNIX, internal.NetUDP:
		return nil, nil
	}

	conf := internal.DefaultTLSConfig()

	cert, ca, err := c.parse()
	if err != nil {
		return nil, err
	}
	if ca != nil {
		conf.RootCAs = ca
	}
	if len(cert.Certificate) >= 0 {
		conf.Certificates = append(conf.Certificates, cert)
	}

	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	conf.ServerName = host
	conf.InsecureSkipVerify = c.InsecureSkipVerify

	switch network {
	case internal.NetQUIC:
		conf.NextProtos = append(conf.NextProtos, "quic")
	}

	return conf, nil
}
