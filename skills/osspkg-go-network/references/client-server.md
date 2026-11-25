# Client and server examples

## Pooled TCP client

The callback owns one connection for the duration of `Call`. A length prefix gives the stream protocol a message boundary:

```go
cli, err := client.New(client.Config{
	Network: "tcp",
	Address: "127.0.0.1:8080",
}, client.WithPoolSize(16))
if err != nil {
	return err
}
defer cli.Close()

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err = cli.Call(ctx, func(rw io.ReadWriter) error {
	payload := []byte("hello")
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := rw.Write(header[:]); err != nil {
		return err
	}
	if _, err := rw.Write(payload); err != nil {
		return err
	}
	if _, err := io.ReadFull(rw, header[:]); err != nil {
		return err
	}
	response := make([]byte, binary.BigEndian.Uint32(header[:]))
	_, err := io.ReadFull(rw, response)
	return err
})
```

Add `client.WithIdleTimeout(duration)` to change the default two-minute idle lifetime. `WithPoolSize` defaults to 32 open connections. A handler error discards that connection; do not keep `rw` after the callback returns.

## Stream server

`server.Handler` gets a context, a stream `io.ReadWriter`, and the remote address. Read complete framed requests and handle multiple messages if the connection protocol permits:

```go
srv, err := server.New(server.Config{
	Network: "tcp",
	Address: "127.0.0.1:8080",
}, server.WithMaxConnections(64))
if err != nil {
	return err
}
srv.HandleFunc(func(ctx context.Context, rw io.ReadWriter, peer net.Addr) error {
	var header [4]byte
	if _, err := io.ReadFull(rw, header[:]); err != nil {
		return err
	}
	payload := make([]byte, binary.BigEndian.Uint32(header[:]))
	if _, err := io.ReadFull(rw, payload); err != nil {
		return err
	}
	if _, err := rw.Write(header[:]); err != nil {
		return err
	}
	_, err := rw.Write(payload)
	return err
})
return srv.ListenAndServe(ctx)
```

Use the handler's context for downstream work. `WithIdleTimeout` applies to TCP and Unix stream connections; UDP has its own message/in-flight limits.

## Framed UDP server handler

UDP handler input is already reassembled into one complete request; response writes are buffered and framed with the original request ID by the server:

```go
srv, err := server.New(server.Config{
	Network: "udp",
	Address: "127.0.0.1:8080",
})
if err != nil {
	return err
}
srv.HandleFunc(func(_ context.Context, rw io.ReadWriter, _ net.Addr) error {
	request, err := io.ReadAll(rw)
	if err != nil {
		return err
	}
	_, err = rw.Write(request)
	return err
})
return srv.ListenAndServe(ctx)
```

UDP supports up to 16 MiB per message and does not retransmit fragments. Use request deadlines at the client.
