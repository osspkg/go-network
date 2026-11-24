/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package cert

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"go.osspkg.com/errors"

	"go.osspkg.com/network/xnet"
)

const (
	EnvNoSystemCA = "NETWORK_NO_SYSTEM_CERT"
)

var (
	ErrInvalidCertificate = errors.New("invalid certificate")
)

type Certificate struct {
	CAFiles  []string `yaml:"ca_files,omitempty" unic:"ca_files,omitempty,desc='Root certificate paths'"`
	CertFile string   `yaml:"crt_file,omitempty" unic:"crt_file,omitempty,desc='Public certificate path'"`
	KeyFile  string   `yaml:"key_file,omitempty" unic:"key_file,omitempty,desc='Private key path'"`
}

func DefaultTLSConfig() *tls.Config {
	return &tls.Config{
		RootCAs:    initializePool(),
		MinVersion: tls.VersionTLS13,
		Rand:       rand.Reader,
	}
}

func initializePool() *x509.CertPool {
	pool := x509.NewCertPool()

	ev, ok := os.LookupEnv(EnvNoSystemCA)
	if !ok {
		return pool
	}
	if ok, _ = strconv.ParseBool(ev); !ok {
		return pool
	}

	spool, err := x509.SystemCertPool()
	if err != nil {
		return pool
	}

	return spool
}

func ParseCertificate(rootCA *x509.CertPool, c Certificate) (*tls.Certificate, error) {
	if len(c.CAFiles) > 0 {
		for _, file := range c.CAFiles {
			caCert, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			rootCA.AppendCertsFromPEM(caCert)
		}
	}

	if len(c.CertFile) > 0 && len(c.KeyFile) > 0 {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, err
		}
		return &cert, nil
	}

	return nil, ErrInvalidCertificate
}

// GenerateRootCertificate creates a self-signed CA certificate and its private key.
// serialNumber must be non-zero, and validity must be positive.
func GenerateRootCertificate(commonName string, serialNumber uint64, validity time.Duration) (*tls.Certificate, error) {
	if strings.TrimSpace(commonName) == "" {
		return nil, errors.New("root certificate requires a common name")
	}
	serial, err := toSerialNumber(serialNumber)
	if err != nil {
		return nil, err
	}
	if validity <= 0 {
		return nil, errors.New("root certificate validity must be positive")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber:          serial,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(validity),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	return makeTLSCertificate(certDER, key)
}

// GenerateServerCertificate creates a server certificate signed by rootCertificate.
// rootCertificate must contain the CA certificate and its matching private key.
// serialNumber must be unique among certificates issued by rootCertificate.
func GenerateServerCertificate(rootCertificate *tls.Certificate, serialNumber uint64, address []string) (*tls.Certificate, error) {
	serial, err := toSerialNumber(serialNumber)
	if err != nil {
		return nil, err
	}
	return generateIssuedCertificate(rootCertificate, serial, address, x509.ExtKeyUsageServerAuth, "")
}

// GenerateClientCertificate creates a client certificate signed by rootCertificate.
// rootCertificate must contain the CA certificate and its matching private key.
// serialNumber must be unique among certificates issued by rootCertificate.
func GenerateClientCertificate(rootCertificate *tls.Certificate, serialNumber uint64) (*tls.Certificate, error) {
	serial, err := toSerialNumber(serialNumber)
	if err != nil {
		return nil, err
	}
	return generateIssuedCertificate(rootCertificate, serial, nil, x509.ExtKeyUsageClientAuth, "client")
}

func generateIssuedCertificate(rootCertificate *tls.Certificate, serialNumber *big.Int, address []string, usage x509.ExtKeyUsage, commonName string) (*tls.Certificate, error) {
	if err := validateSerialNumber(serialNumber); err != nil {
		return nil, err
	}

	issuer, signer, err := parseCertificateAuthority(rootCertificate)
	if err != nil {
		return nil, err
	}

	return generateCertificate(issuer, signer, serialNumber, address, usage, commonName)
}

func generateSelfSignedServerCertificate(address []string) (*tls.Certificate, error) {
	serialNumber, err := generateSerialNumber()
	if err != nil {
		return nil, err
	}

	return generateCertificate(nil, nil, serialNumber, address, x509.ExtKeyUsageServerAuth, "")
}

func generateCertificate(issuer *x509.Certificate, signer crypto.Signer, serialNumber *big.Int, address []string, usage x509.ExtKeyUsage, commonName string) (*tls.Certificate, error) {
	if err := validateSerialNumber(serialNumber); err != nil {
		return nil, err
	}

	ips, domains := xnet.SplitAddressType(address)
	if len(ips)+len(domains) == 0 && commonName == "" {
		return nil, errors.New("certificate requires at least one DNS name or IP address")
	}
	for _, domain := range domains {
		probe := domain
		if strings.HasPrefix(domain, "*.") {
			probe = "autogen" + domain[1:]
		}
		if err := (&x509.Certificate{DNSNames: []string{domain}}).VerifyHostname(probe); err != nil {
			return nil, errors.Wrapf(err, "invalid certificate DNS name %q", domain)
		}
	}

	if len(domains) > 0 {
		commonName = domains[0]
	} else if len(ips) > 0 {
		commonName = ips[0].String()
	}

	now := time.Now().UTC()
	notBefore := now.Add(-5 * time.Minute)
	notAfter := now.AddDate(0, 0, 30)
	if issuer != nil {
		if notBefore.Before(issuer.NotBefore) {
			notBefore = issuer.NotBefore
		}
		if notAfter.After(issuer.NotAfter) {
			notAfter = issuer.NotAfter
		}
		if !notAfter.After(notBefore) {
			return nil, errors.New("root certificate expires before the generated certificate can be valid")
		}
	}

	template := &x509.Certificate{
		SerialNumber:          new(big.Int).Set(serialNumber),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{usage},
		BasicConstraintsValid: true,
		Subject:               pkix.Name{CommonName: commonName},
		IPAddresses:           ips,
		DNSNames:              domains,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	parent := issuer
	if parent == nil {
		parent = template
		signer = key
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, signer)
	if err != nil {
		return nil, err
	}

	return makeTLSCertificate(certDER, key)
}

func makeTLSCertificate(certDER []byte, key *ecdsa.PrivateKey) (*tls.Certificate, error) {
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	crt, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	crt.Leaf, err = x509.ParseCertificate(certDER)
	if err != nil {
		return nil, err
	}

	return &crt, nil
}

func parseCertificateAuthority(certificate *tls.Certificate) (*x509.Certificate, crypto.Signer, error) {
	if certificate == nil || len(certificate.Certificate) == 0 || certificate.PrivateKey == nil {
		return nil, nil, errors.New("root certificate and private key are required")
	}

	issuer, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return nil, nil, errors.Wrapf(err, "parse root certificate")
	}
	if !issuer.BasicConstraintsValid || !issuer.IsCA || issuer.KeyUsage&x509.KeyUsageCertSign == 0 {
		return nil, nil, errors.New("root certificate is not authorized to sign certificates")
	}

	signer, ok := certificate.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, nil, errors.New("root certificate private key cannot sign certificates")
	}
	certificatePublicKey, err := x509.MarshalPKIXPublicKey(issuer.PublicKey)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "marshal root certificate public key")
	}
	signerPublicKey, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return nil, nil, errors.Wrapf(err, "marshal root private key public key")
	}
	if !bytes.Equal(certificatePublicKey, signerPublicKey) {
		return nil, nil, errors.New("root certificate does not match its private key")
	}

	now := time.Now()
	if now.Before(issuer.NotBefore) || !now.Before(issuer.NotAfter) {
		return nil, nil, errors.New("root certificate is not currently valid")
	}

	return issuer, signer, nil
}

func validateSerialNumber(serialNumber *big.Int) error {
	if serialNumber == nil || serialNumber.Sign() <= 0 || serialNumber.BitLen() > 159 {
		return errors.New("certificate serial number must be positive and no longer than 20 octets")
	}
	return nil
}

func toSerialNumber(serialNumber uint64) (*big.Int, error) {
	if serialNumber == 0 {
		return nil, errors.New("certificate serial number must be positive")
	}
	return new(big.Int).SetUint64(serialNumber), nil
}

func generateSerialNumber() (*big.Int, error) {
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 159)
	serialNumber, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, errors.Wrapf(err, "generate certificate serial number")
	}
	for serialNumber.Sign() == 0 {
		serialNumber, err = rand.Int(rand.Reader, serialLimit)
		if err != nil {
			return nil, errors.Wrapf(err, "generate certificate serial number")
		}
	}
	return serialNumber, nil
}
