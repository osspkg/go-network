# client

The `client` package provides a client for running requests over reusable network connections. `Call` can be invoked concurrently; the pool size limits the total number of open connections.

TCP, UDP, and Unix domain sockets are supported. TLS can be configured for TCP with `cert.ClientConfig`. UDP uses the internal `GONETUD1` datagram format: the remote peer must support this format and echo the request ID in its response.

## Creating a client

```go
cli, err := client.New(client.Config{
	Address: "127.0.0.1:8080",
	Network: "tcp",
},
	client.WithPoolSize(16),
	client.WithIdleTimeout(time.Minute),
)
if err != nil {
	return err
}
defer cli.Close()
```

`Network` accepts `"tcp"`, `"udp"`, or `"unix"`. TCP and UDP addresses use the `host:port` format; Unix addresses are socket paths. `Config` does not populate the address or network automatically, so set both explicitly.

Pool options:

- `WithPoolSize(size)` sets the maximum number of open connections. The default is 32.
- `WithIdleTimeout(timeout)` sets how long an unused connection remains in the pool. The default is 2 minutes.

Both values must be positive.

## Running a request

`Call` borrows one connection for the duration of the handler. The handler receives an `io.ReadWriter` to write the request and read the response:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err = cli.Call(ctx, func(rw io.ReadWriter) error {
	if _, err := io.WriteString(rw, "ping"); err != nil {
		return err
	}

	response := make([]byte, 4)
	if _, err := io.ReadFull(rw, response); err != nil {
		return err
	}
	if string(response) != "pong" {
		return fmt.Errorf("unexpected response: %q", response)
	}
	return nil
})
```

For stream connections (TCP and Unix), the handler must read the complete response before returning so the connection can be safely reused. Do not retain `rw` after the handler returns. If the handler returns an error, the connection is closed and removed from the pool.

The context bounds waiting for an available connection and connection establishment. If it is canceled during a call, the active connection is closed, which interrupts network I/O. A handler doing CPU work or waiting on unrelated resources must check the captured context itself; Go cannot forcibly stop the handler. For UDP, the first read sends the buffered request; if the handler does not read, `Call` sends it after the handler returns. Read the response according to the remote peer's protocol.

`Close` closes active and idle connections. Subsequent calls to `Call` return `client.ErrClientClosed`. Close the client when it is no longer needed.
