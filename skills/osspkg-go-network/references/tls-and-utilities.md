# TLS and utility packages

## TLS certificates

Use `cert.ClientConfig.Parse` for client TLS and `cert.ServerConfig.Parse` for server TLS. TLS transport integration in `client` and `server` is supported only for TCP.

```go
root, err := cert.GenerateRootCertificate("Example CA", 1, 365*24*time.Hour)
if err != nil {
	return err
}
serverCert, err := cert.GenerateServerCertificate(root, 2, []string{"api.example.test"})
if err != nil {
	return err
}
clientCert, err := cert.GenerateClientCertificate(root, 3)
if err != nil {
	return err
}
```

Leaf serial numbers must be non-zero and unique under the CA. Server certificates accept DNS names and IPs for SANs; client certificates do not take addresses. Generated leaf certificates last up to 30 days, bounded by the CA's validity. Keep private keys secret.

`Certificate.CAFiles` populate `tls.Config.RootCAs`. In `ClientConfig`, those roots verify the server. `ServerConfig.Certs` does not configure `ClientCAs` or enable client certificate authentication. `InsecureSkipVerify` disables standard server verification; use it only with an explicit alternative verification mechanism.

## Listener and address helpers

- `listen.New(ctx, "tcp", address, tlsConfig)` returns an `io.Closer` that implements `net.Listener`; for UDP it implements `net.PacketConn`; for Unix stream sockets it implements `net.Listener`.
- `xnet.ResolveIPPort` normalizes host/port input, defaulting to `127.0.0.1` and port `8080` where omitted. IPv6 with a port must be bracketed.
- `xnet.ListenRandomPort(host)` returns a TCP listener that keeps the chosen port reserved. `RandomPort` closes the listener before returning the address, so the port can be claimed by another process before use.
- `xnet.FixIPPort` filters to IP addresses and fills missing/zero ports. `SplitAddressType` separates parsed IPs from non-IP strings; it does not validate DNS names.

## File descriptors

`fd.ByConnect(net.Conn)` uses `syscall.Conn.SyscallConn().Control`; `fd.BySyscallConn` accepts that interface directly; `fd.ByFile` returns `(*os.File).Fd()`. All return `-1` when no descriptor is available. The descriptor is borrowed and must not outlive the open connection or file.

## Linux epoll server

`epoll.New` creates a Linux-only TCP reactor. Callbacks execute synchronously on its event loop; they must be brief and non-blocking. `OnData` receives arbitrary stream chunks, so framing is up to the application. Queue output through `Conn.Write`; `Conn.Close` schedules closure. Non-Linux builds provide a stub whose `ListenAndServe` reports that epoll is unsupported.

```go
srv, err := epoll.New(epoll.Config{
	Address: ":8080",
	OnData: func(conn *epoll.Conn, data []byte) error {
		return conn.Write(data)
	},
})
if err != nil {
	return err
}
return srv.ListenAndServe(ctx)
```
