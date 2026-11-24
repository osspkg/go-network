/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package cert

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultTLSConfig(t *testing.T) {
	t.Setenv(EnvNoSystemCA, "false")
	got := DefaultTLSConfig()
	if got == nil || got.RootCAs == nil {
		t.Fatal("DefaultTLSConfig() returned a nil config or root pool")
	}
	if got.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion = %d, want TLS 1.3", got.MinVersion)
	}
	if got.Rand == nil {
		t.Fatal("Rand is nil")
	}
}

func TestParseCertificate(t *testing.T) {
	root, err := GenerateRootCertificate("test CA", 1, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := GenerateServerCertificate(root, 2, []string{"example.test"})
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := writeTLSCertificate(t, leaf)
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: root.Certificate[0]}), 0o600); err != nil {
		t.Fatal(err)
	}

	pool := x509.NewCertPool()
	got, err := ParseCertificate(pool, Certificate{CAFiles: []string{caPath}, CertFile: certPath, KeyFile: keyPath})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Leaf == nil {
		t.Fatal("ParseCertificate() returned no parsed certificate")
	}
	pool.AddCert(root.Leaf)
	if _, err := got.Leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: "example.test", KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		t.Fatalf("loaded CA does not verify the loaded certificate: %v", err)
	}
}

func TestParseCertificateErrors(t *testing.T) {
	if got, err := ParseCertificate(x509.NewCertPool(), Certificate{}); got != nil || err != ErrInvalidCertificate {
		t.Fatalf("ParseCertificate(empty) = (%v, %v), want (nil, ErrInvalidCertificate)", got, err)
	}
	if _, err := ParseCertificate(x509.NewCertPool(), Certificate{CAFiles: []string{"missing-ca.pem"}}); err == nil {
		t.Fatal("ParseCertificate() with missing CA file succeeded")
	}

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseCertificate(x509.NewCertPool(), Certificate{CertFile: certPath, KeyFile: keyPath}); err == nil {
		t.Fatal("ParseCertificate() with invalid key pair succeeded")
	}
}

func TestClientConfigParse(t *testing.T) {
	if _, err := (*ClientConfig)(nil).Parse(); err == nil {
		t.Fatal("nil ClientConfig.Parse() succeeded")
	}

	root, err := GenerateRootCertificate("client test CA", 11, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	client, err := GenerateClientCertificate(root, 12)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := writeTLSCertificate(t, client)
	caPath := writeCAPEM(t, root)

	got, err := (&ClientConfig{Certs: []Certificate{{CAFiles: []string{caPath}, CertFile: certPath, KeyFile: keyPath}}, InsecureSkipVerify: true}).Parse()
	if err != nil {
		t.Fatal(err)
	}
	if !got.InsecureSkipVerify || len(got.Certificates) != 1 || got.MinVersion != tls.VersionTLS13 {
		t.Fatalf("unexpected client TLS config: insecure=%v certs=%d minVersion=%d", got.InsecureSkipVerify, len(got.Certificates), got.MinVersion)
	}
	if _, err := got.Certificates[0].Leaf.Verify(x509.VerifyOptions{Roots: got.RootCAs, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Fatalf("client CA pool does not verify the configured client certificate: %v", err)
	}
}

func TestServerConfigParse(t *testing.T) {
	if _, err := (*ServerConfig)(nil).Parse(); err == nil {
		t.Fatal("nil ServerConfig.Parse() succeeded")
	}

	root, err := GenerateRootCertificate("server test CA", 21, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	server, err := GenerateServerCertificate(root, 22, []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := writeTLSCertificate(t, server)

	got, err := (&ServerConfig{Certs: []Certificate{{CertFile: certPath, KeyFile: keyPath}}, NextProtos: []string{"h2"}, Autogen: []string{"generated.example.test"}}).Parse()
	if err != nil {
		t.Fatal(err)
	}
	if got.MinVersion != tls.VersionTLS13 || len(got.Certificates) != 2 || len(got.NextProtos) != 1 || got.NextProtos[0] != "h2" {
		t.Fatalf("unexpected server TLS config: minVersion=%d certs=%d nextProtos=%v", got.MinVersion, len(got.Certificates), got.NextProtos)
	}
	if got.Certificates[0].Leaf == nil || got.Certificates[0].Leaf.DNSNames[0] != "localhost" {
		t.Fatalf("configured server certificate was not loaded: %#v", got.Certificates[0].Leaf)
	}
	generated := got.Certificates[1].Leaf
	if generated == nil || len(generated.DNSNames) != 1 || generated.DNSNames[0] != "generated.example.test" {
		t.Fatalf("autogenerated certificate has unexpected SANs: %#v", generated)
	}
}

func TestGenerateCertificatesAndValidateChains(t *testing.T) {
	root, err := GenerateRootCertificate("  Example Root  ", 31, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	rootLeaf, err := x509.ParseCertificate(root.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if !rootLeaf.IsCA || rootLeaf.Subject.CommonName != "  Example Root  " || rootLeaf.SerialNumber.Uint64() != 31 {
		t.Fatalf("unexpected root certificate: %#v", rootLeaf)
	}

	server, err := GenerateServerCertificate(root, 32, []string{"api.example.test", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	client, err := GenerateClientCertificate(root, 33)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(rootLeaf)
	verify := func(c *tls.Certificate, usage x509.ExtKeyUsage, name string) {
		t.Helper()
		if c.Leaf == nil {
			t.Fatal("generated leaf is not parsed")
		}
		_, err := c.Leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: name, KeyUsages: []x509.ExtKeyUsage{usage}})
		if err != nil {
			t.Fatalf("certificate verification failed: %v", err)
		}
	}
	verify(server, x509.ExtKeyUsageServerAuth, "api.example.test")
	verify(server, x509.ExtKeyUsageServerAuth, "127.0.0.1")
	verify(client, x509.ExtKeyUsageClientAuth, "")
	if !strings.Contains(client.Leaf.Subject.CommonName, "client") {
		t.Fatalf("client common name = %q, want client", client.Leaf.Subject.CommonName)
	}
}

func TestGenerateCertificateValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		common   string
		serial   uint64
		validity time.Duration
	}{
		{name: "blank common name", common: "  ", serial: 1, validity: time.Hour},
		{name: "zero serial", common: "root", serial: 0, validity: time.Hour},
		{name: "zero validity", common: "root", serial: 1, validity: 0},
		{name: "negative validity", common: "root", serial: 1, validity: -time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := GenerateRootCertificate(tc.common, tc.serial, tc.validity); got != nil || err == nil {
				t.Fatalf("GenerateRootCertificate() = (%v, %v), want error", got, err)
			}
		})
	}

	root, err := GenerateRootCertificate("validation CA", 41, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		serial  uint64
		address []string
	}{
		{name: "zero server serial", serial: 0, address: []string{"example.test"}},
		{name: "no server name", serial: 42},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := GenerateServerCertificate(root, tc.serial, tc.address); got != nil || err == nil {
				t.Fatalf("GenerateServerCertificate() = (%v, %v), want error", got, err)
			}
		})
	}
	if got, err := GenerateClientCertificate(root, 0); got != nil || err == nil {
		t.Fatalf("GenerateClientCertificate(zero serial) = (%v, %v), want error", got, err)
	}
	if got, err := GenerateServerCertificate(nil, 44, []string{"example.test"}); got != nil || err == nil {
		t.Fatalf("GenerateServerCertificate(nil CA) = (%v, %v), want error", got, err)
	}
}

func TestParseCertificateAuthorityRejectsMismatches(t *testing.T) {
	root, err := GenerateRootCertificate("authority test", 51, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	other, err := GenerateRootCertificate("other authority", 52, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		cert *tls.Certificate
	}{
		{name: "nil"},
		{name: "missing private key", cert: &tls.Certificate{Certificate: root.Certificate}},
		{name: "mismatched private key", cert: &tls.Certificate{Certificate: root.Certificate, PrivateKey: other.PrivateKey}},
		{name: "non-CA certificate", cert: func() *tls.Certificate {
			leaf, e := GenerateServerCertificate(root, 53, []string{"example.test"})
			if e != nil {
				t.Fatal(e)
			}
			return leaf
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := parseCertificateAuthority(tc.cert); err == nil {
				t.Fatal("parseCertificateAuthority() succeeded for invalid CA")
			}
		})
	}
}

func TestValidateSerialNumber(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   *big.Int
		want bool
	}{
		{name: "nil"},
		{name: "zero", in: new(big.Int)},
		{name: "negative", in: big.NewInt(-1)},
		{name: "too large", in: new(big.Int).Lsh(big.NewInt(1), 159)},
		{name: "maximum valid", in: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 159), big.NewInt(1)), want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSerialNumber(tc.in)
			if (err == nil) != tc.want {
				t.Fatalf("validateSerialNumber(%v) error = %v, want success=%v", tc.in, err, tc.want)
			}
		})
	}
}

func writeTLSCertificate(t *testing.T, cert *tls.Certificate) (string, string) {
	t.Helper()
	if len(cert.Certificate) == 0 {
		t.Fatal("certificate chain is empty")
	}
	key, ok := cert.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("private key type = %T, want *ecdsa.PrivateKey", cert.PrivateKey)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func writeCAPEM(t *testing.T, cert *tls.Certificate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
