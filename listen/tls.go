/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package listen

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

	"go.osspkg.com/network/internal"
)

type SSL struct {
	Certs      []Certificate
	NextProtos []string
}

type Certificate struct {
	CAFile       string   `yaml:"ca_file"`
	CertFile     string   `yaml:"cert_file"`
	KeyFile      string   `yaml:"key_file"`
	Addresses    []string `yaml:"addresses"`
	AutoGenerate bool     `yaml:"auto_generate"`
}

func NewTLSConfig(ssl *SSL) (*tls.Config, error) {
	rootCA := x509.NewCertPool()
	certificates := make([]tls.Certificate, 0, len(ssl.Certs))

	var (
		c   tls.Certificate
		err error
	)
	for _, cert := range ssl.Certs {
		if cert.AutoGenerate {
			c, err = generateCertificate(cert.Addresses)
		} else {
			c, err = parseCertificate(rootCA, cert)
		}
		if err != nil {
			return nil, err
		}
		certificates = append(certificates, c)
	}

	config := internal.DefaultTLSConfig()
	config.Certificates = certificates
	config.RootCAs = rootCA
	config.NextProtos = append(config.NextProtos, ssl.NextProtos...)
	return config, nil
}

func parseCertificate(rootCA *x509.CertPool, c Certificate) (cert tls.Certificate, err error) {
	if len(c.CertFile) > 0 || len(c.KeyFile) > 0 {
		cert, err = tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return
		}
	}
	if len(c.CAFile) > 0 {
		if caCert, e := os.ReadFile(c.CAFile); e == nil {
			rootCA.AppendCertsFromPEM(caCert)
		}
	}
	return
}

func dnsNames(addresses []string) (ips []net.IP, domains []string) {
	if len(addresses) == 0 {
		ips = append(ips, net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0"))
		domains = append(domains, "localhost")
		if san, err := os.Hostname(); err != nil {
			domains = append(domains, san)
		}
		return
	}
	for _, address := range addresses {
		san, _, err := net.SplitHostPort(address) //nolint: errcheck
		if err == nil {
			ips = append(ips, net.ParseIP(san))
			continue
		}
		domains = append(domains, address)
	}
	return
}

func generateCertificate(address []string) (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return tls.Certificate{}, err
	}

	commonName := "*"
	ips, domains := dnsNames(address)
	if len(domains) > 0 {
		commonName = domains[0]
	}

	template := &x509.Certificate{
		SerialNumber:                big.NewInt(1),
		KeyUsage:                    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		ExtKeyUsage:                 []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		Subject:                     pkix.Name{CommonName: commonName},
		IPAddresses:                 ips,
		DNSNames:                    domains,
		PermittedDNSDomainsCritical: true,
		NotBefore:                   time.Now().UTC(),
		NotAfter:                    time.Now().Add(time.Hour * 24 * 365 * 2).UTC(),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	return tls.X509KeyPair(certPEM, keyPEM)
}
