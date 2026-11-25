# listen

The `listen` package creates network listeners for TCP, UDP, and Unix domain sockets. Its `New` function returns an `io.Closer`, allowing callers to close any supported listener through one common interface.

## Create a TCP listener

```go
listener, err := listen.New(ctx, "tcp", "127.0.0.1:8080", nil)
if err != nil {
	return err
}
defer listener.Close()

tcpListener, ok := listener.(net.Listener)
if !ok {
	return errors.New("expected a stream listener")
}
conn, err := tcpListener.Accept()
```

TCP addresses use Go's `host:port` format. To let the operating system choose a port, use port `0` and inspect `Addr()` on the returned `net.Listener`.

## Create a UDP packet listener

For UDP, `New` returns a `net.PacketConn`:

```go
packetListener, err := listen.New(ctx, "udp", "127.0.0.1:8080", nil)
if err != nil {
	return err
}
defer packetListener.Close()

packetConn := packetListener.(net.PacketConn)
buffer := make([]byte, 1200)
n, peer, err := packetConn.ReadFrom(buffer)
```

Use `ReadFrom` and `WriteTo` to receive packets and reply to their source addresses. When working with this module's framed UDP protocol, see the `proto/udpframe` package.

## Enable TLS for TCP

Pass a `*tls.Config` to enable TLS. The returned value remains a `net.Listener`; accepted connections perform TLS handshakes when used:

```go
listener, err := listen.New(ctx, "tcp", ":8443", tlsConfig)
if err != nil {
	return err
}
defer listener.Close()

tcpListener := listener.(net.Listener)
conn, err := tcpListener.Accept()
```

TLS is only applied to TCP. The UDP and Unix cases do not use the supplied TLS configuration.

## Unix domain sockets

Use the `"unix"` network name and provide a filesystem path:

```go
listener, err := listen.New(ctx, "unix", "/run/my-service.sock", nil)
if err != nil {
	return err
}
defer listener.Close()

unixListener := listener.(net.Listener)
conn, err := unixListener.Accept()
```

`New` accepts the network names `"tcp"`, `"udp"`, and `"unix"`. Any other network name or an invalid address returns an error. The supplied context is passed to Go's `net.ListenConfig` and can cancel listener creation.
