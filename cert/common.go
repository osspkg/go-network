/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"time"
)

type Certificate struct {
	CAFile   string `yaml:"ca_file,omitempty" unic:"ca_file,omitempty,desc='Адрес корневого сертификата'"`
	CertFile string `yaml:"cert_file,omitempty" unic:"cert_file,omitempty,desc='Адрес публичного сертификата'"`
	KeyFile  string `yaml:"key_file,omitempty" unic:"key_file,omitempty,desc='Адрес приватного ключа'"`
}

func DefaultTLSConfig() *tls.Config {
	return &tls.Config{
		RootCAs:    x509.NewCertPool(),
		MinVersion: tls.VersionTLS13,
		Rand:       rand.Reader,
		CurvePreferences: []tls.CurveID{
			tls.CurveP521,
			tls.CurveP384,
			tls.CurveP256,
			tls.X25519,
		},
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}
}

func SplitAddress(addresses []string) (ips []net.IP, domains []string) {
	for _, address := range addresses {
		if san, _, err := net.SplitHostPort(address); err == nil {
			ips = append(ips, net.ParseIP(san))
			continue
		}
		domains = append(domains, address)
	}
	return
}

func ParseCertificate(rootCA *x509.CertPool, c Certificate) (*tls.Certificate, error) {
	if len(c.CAFile) > 0 {
		caCert, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, err
		}
		rootCA.AppendCertsFromPEM(caCert)
	}

	if len(c.CertFile) > 0 || len(c.KeyFile) > 0 {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, err
		}
		return &cert, nil
	}

	return nil, nil
}

func GenerateCertificate(address []string) (*tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	commonName := "*"
	ips, domains := SplitAddress(address)
	if len(domains) > 0 {
		commonName = domains[0]
	}

	now := time.Now()

	template := &x509.Certificate{
		SerialNumber:                big.NewInt(now.UnixMilli()),
		KeyUsage:                    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		ExtKeyUsage:                 []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		Subject:                     pkix.Name{CommonName: commonName},
		IPAddresses:                 ips,
		DNSNames:                    domains,
		PermittedDNSDomainsCritical: true,
		NotBefore:                   now.UTC(),
		NotAfter:                    now.AddDate(0, 0, 30).UTC(),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	crt, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &crt, nil
}
