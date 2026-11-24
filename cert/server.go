/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package cert

import (
	"crypto/tls"

	"go.osspkg.com/errors"
)

type ServerConfig struct {
	Certs      []Certificate `yaml:"certs,omitempty" unic:"certs,omitempty,desc='List of certificates'"`
	NextProtos []string      `yaml:"next_protos,omitempty" unic:"next_protos,omitempty,desc='Protocol negotiation at the application layer'"`
	Autogen    []string      `yaml:"autogen,omitempty" unic:"autogen,omitempty,desc='List of addresses for automatic certificate generation'"`
}

func (v *ServerConfig) Parse() (*tls.Config, error) {
	if v == nil {
		return nil, errors.New("server config is nil")
	}

	config := DefaultTLSConfig()
	config.NextProtos = append(config.NextProtos, v.NextProtos...)

	for _, cert := range v.Certs {
		c, err := ParseCertificate(config.RootCAs, cert)
		if err != nil {
			return nil, err
		}
		if c == nil {
			return nil, ErrInvalidCertificate
		}
		config.Certificates = append(config.Certificates, *c)
	}

	if len(v.Autogen) > 0 {
		c, err := generateSelfSignedServerCertificate(v.Autogen)
		if err != nil {
			return nil, err
		}
		if c == nil {
			return nil, ErrInvalidCertificate
		}
		config.Certificates = append(config.Certificates, *c)
	}

	return config, nil
}
