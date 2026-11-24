/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package cert

import (
	"crypto/tls"

	"go.osspkg.com/errors"
)

type ClientConfig struct {
	Certs              []Certificate `yaml:"certs,omitempty" unic:"certs,omitempty,desc='List of certificates'"`
	InsecureSkipVerify bool          `yaml:"insecure_skip_verify,omitempty" unic:"insecure_skip_verify,omitempty,default='false',desc='Skip root certificate verification'"`
}

func (v *ClientConfig) Parse() (*tls.Config, error) {
	if v == nil {
		return nil, errors.New("client config is nil")
	}

	config := DefaultTLSConfig()

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

	config.InsecureSkipVerify = v.InsecureSkipVerify

	return config, nil
}
