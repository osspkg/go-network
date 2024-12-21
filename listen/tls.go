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

	"go.osspkg.com/network/internal"
)

type Certificate struct {
	CAFile       string   `yaml:"ca_file"`
	CertFile     string   `yaml:"cert_file"`
	KeyFile      string   `yaml:"key_file"`
	Addresses    []string `yaml:"addresses"`
	AutoGenerate bool     `yaml:"auto_generate"`
}

func tlsConfig(certs ...Certificate) (*tls.Config, error) {
	rootCA := x509.NewCertPool()
	certificates := make([]tls.Certificate, 0, len(certs))

	var (
		c   tls.Certificate
		err error
	)
	for _, cert := range certs {
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
		domains = append(domains, "localhost", address)
	}
	return
}

func generateCertificate(address []string) (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return tls.Certificate{}, err
	}

	ips, domains := dnsNames(address)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "*"},
		IPAddresses:  ips,
		DNSNames:     domains,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	return tls.X509KeyPair(certPEM, keyPEM)
}
