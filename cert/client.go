/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package cert

import (
	"crypto/tls"

	"go.osspkg.com/errors"

	"go.osspkg.com/network/errs"
)

type ClientConfig struct {
	Certs              []Certificate `yaml:"certs,omitempty" unic:"certs,omitempty,desc='Список сертификатов'"`
	InsecureSkipVerify bool          `yaml:"insecure_skip_verify,omitempty" unic:"insecure_skip_verify,omitempty,default='false',desc='Пропускать проверку корневых сертификатов'"`
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
			return nil, errs.ErrInvalidCertificate
		}
		config.Certificates = append(config.Certificates, *c)
	}

	config.InsecureSkipVerify = v.InsecureSkipVerify

	return config, nil
}
