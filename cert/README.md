# cert

Package `cert` loads X.509 certificates and builds TLS configurations for the
network clients and servers. It also provides helpers for creating a private
certificate authority (CA) and issuing server and client certificates.

## Loading certificates

`Certificate` describes certificate files:

```go
certFiles := cert.Certificate{
	CAFiles:  []string{"certs/root-ca.pem"},
	CertFile: "certs/server.pem",
	KeyFile:  "certs/server-key.pem",
}
```

`ParseCertificate` appends certificates from `CAFiles` to the supplied
`*x509.CertPool` and loads the certificate and private key as a TLS key pair.
Both `CertFile` and `KeyFile` are required to return a key pair. File paths
must point to PEM-encoded data accepted by Go's TLS implementation.

`DefaultTLSConfig` creates a TLS configuration with TLS 1.3 as the minimum
version. By default its root pool is empty. If the `NETWORK_NO_SYSTEM_CERT`
environment variable parses as `true`, it uses the system certificate pool
when available.

## Client and server configuration

`ClientConfig.Parse` and `ServerConfig.Parse` return a `*tls.Config` and an
error. Both accept lists of `Certificate` values and append their CA files to
the resulting `tls.Config.RootCAs` pool. In `ClientConfig`, this pool is used
to verify server certificates. Adding a CA through `ServerConfig.Certs` does
not configure `tls.Config.ClientCAs` or enable client-certificate
verification. A client can set `InsecureSkipVerify` to disable normal server
certificate verification; this should be used only when the application
supplies an alternative verification mechanism.

`ServerConfig` also accepts `NextProtos` for application protocol negotiation.
Its `Autogen` field generates a self-signed server certificate for the listed
DNS names and IP addresses when non-empty. This certificate is separate from
the key pairs loaded from `Certs` and is not signed by a configured CA.

Example:

```go
tlsConfig, err := (&cert.ServerConfig{
	Certs: []cert.Certificate{{
		CertFile: "certs/server.pem",
		KeyFile:  "certs/server-key.pem",
	}},
	NextProtos: []string{"h2", "http/1.1"},
}).Parse()
if err != nil {
	return err
}
```

## Generating a CA and certificates

`GenerateRootCertificate` creates a self-signed CA certificate and private
key. `GenerateServerCertificate` and `GenerateClientCertificate` issue leaf
certificates signed by that CA. The returned `*tls.Certificate` for the CA
must be kept with its private key to issue leaf certificates.

```go
root, err := cert.GenerateRootCertificate("Example CA", 1, 365*24*time.Hour)
if err != nil {
	return err
}

server, err := cert.GenerateServerCertificate(root, 2, []string{"api.example.test"})
if err != nil {
	return err
}

client, err := cert.GenerateClientCertificate(root, 3)
if err != nil {
	return err
}
```

The root common name must not be blank, its serial number must be non-zero, and
its validity must be positive. Leaf serial numbers must also be non-zero and
unique among certificates issued by the same CA. The server generator accepts
DNS names and IP addresses for its Subject Alternative Name; the client
generator does not take addresses. Generated leaf certificates are valid for
up to 30 days, bounded by the CA's validity period. Keep generated private
keys secret.

The package also exposes `ErrInvalidCertificate` when a certificate
configuration does not provide a certificate/key pair.
