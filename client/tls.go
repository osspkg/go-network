/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"crypto/tls"
	"crypto/x509"
	"os"
)

type Certificate struct {
	CAFile             string `yaml:"ca_file"`
	CertFile           string `yaml:"cert_file"`
	KeyFile            string `yaml:"key_file"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

func parseCertificate(c Certificate) (cert tls.Certificate, ca *x509.CertPool, err error) {
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
