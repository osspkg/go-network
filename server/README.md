# server

The `server` package runs a handler over TCP, UDP, or a Unix domain socket. A handler receives a context, an `io.ReadWriter`, and the remote address. TCP and Unix handlers own the stream protocol and may process multiple requests on one connection. UDP handlers receive one complete framed message and produce one response message.

## Create and run a TCP server

```go
srv, err := server.New(server.Config{
	Network: "tcp",
	Address: "127.0.0.1:8080",
}, server.WithMaxConnections(64))
if err != nil {
	return err
}

srv.HandleFunc(echo) // stream protocol example shown below

return srv.ListenAndServe(ctx)
```

Call `HandleFunc` before `ListenAndServe`. The server starts listening when `ListenAndServe` is called and returns when the context is canceled, `Close` is called, or the listener fails. Closing the server closes its listener and active stream connections, and waits for handlers to return before `ListenAndServe` exits.

## Stream handler example

TCP and Unix connections are byte streams, so the handler and client must agree on message boundaries. This example uses a four-byte big-endian length followed by the payload and can handle multiple requests per connection:

```go
func echo(ctx context.Context, rw io.ReadWriter, addr net.Addr) error {
	for {
		var header [4]byte
		if _, err := io.ReadFull(rw, header[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return nil // peer closed the connection between requests
			}
			return err
		}

		size := binary.BigEndian.Uint32(header[:])
		payload := make([]byte, size)
		if _, err := io.ReadFull(rw, payload); err != nil {
			return err
		}
		if _, err := rw.Write(header[:]); err != nil {
			return err
		}
		if _, err := rw.Write(payload); err != nil {
			return err
		}
	}
}
```

The handler context is canceled when the server closes. Use it to stop application work and pass it to downstream calls. `addr` identifies the peer and can be used for logging or request-specific policy.

## UDP handler

UDP uses the `GONETUD1` framing implemented by `proto/udpframe`. The transport reassembles each request before invoking the handler; the handler reads one complete request message and writes one complete response message:

```go
func udpEcho(_ context.Context, rw io.ReadWriter, addr net.Addr) error {
	request, err := io.ReadAll(rw)
	if err != nil {
		return err
	}
	_, err = rw.Write(request)
	return err
}

srv, err := server.New(server.Config{Network: "udp", Address: "127.0.0.1:8080"})
if err != nil {
	return err
}
srv.HandleFunc(udpEcho)
return srv.ListenAndServe(ctx)
```

UDP messages can be up to 16 MiB and are split into datagrams of at most 1200 bytes. The protocol does not retransmit lost fragments; use request deadlines and account for UDP packet loss. See `proto/udpframe/README.md` for the wire format and client-side framing API.

## Configuration and options

`Config.Network` accepts `"tcp"`, `"udp"`, or `"unix"`. TCP and UDP addresses use `host:port`; Unix addresses are socket paths. For TCP, set `Certificate` to a `*cert.ServerConfig` to enable TLS:

```go
srv, err := server.New(server.Config{
	Network: "tcp",
	Address: ":8443",
	Certificate: &cert.ServerConfig{
		Certs: []cert.Certificate{{
			CertFile: "server.pem",
			KeyFile:  "server-key.pem",
		}},
	},
})
```

TLS configuration is only supported with TCP. Certificate loading and automatic certificate generation are documented in the `cert` package.

- `WithMaxConnections(limit)` limits simultaneous TCP/Unix handlers or UDP handlers. The default is the smaller of `GOMAXPROCS` and 256; valid values are 1 through 4096.
- `WithIdleTimeout(timeout)` closes an idle TCP or Unix connection after the configured duration. The default is two minutes, and the timeout must be positive.

`New` validates the network, address, certificate configuration, and options, but does not start listening. Each server instance can be started once. `Close` is safe to call to stop a running server; after closing, it cannot be restarted.
