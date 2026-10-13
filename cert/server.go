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

type ServerConfig struct {
	Certs      []Certificate `yaml:"certs,omitempty" unic:"certs,omitempty,desc='Список сертификатов'"`
	NextProtos []string      `yaml:"next_protos,omitempty" unic:"next_protos,omitempty,desc='Согласование протоколов на прикладном уровне'"`
	Autogen    []string      `yaml:"autogen,omitempty" unic:"autogen,omitempty,desc='Список адресов для автогенерации сертификатов'"`
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
			return nil, errs.ErrInvalidCertificate
		}
		config.Certificates = append(config.Certificates, *c)
	}

	if len(v.Autogen) > 0 {
		c, err := GenerateCertificate(v.Autogen)
		if err != nil {
			return nil, err
		}
		if c == nil {
			return nil, errs.ErrInvalidCertificate
		}
		config.Certificates = append(config.Certificates, *c)
	}

	return config, nil
}
