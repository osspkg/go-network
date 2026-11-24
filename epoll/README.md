# epoll

The `epoll` package provides a Linux TCP server that dispatches socket readiness through one epoll event loop. It is intended for event-driven protocols where callbacks can process available bytes quickly. The package builds on non-Linux platforms, but `ListenAndServe` returns an unsupported-platform error there.

## Echo server

```go
srv, err := epoll.New(epoll.Config{
	Address: ":8080",
	OnConnect: func(conn *epoll.Conn) error {
		log.Printf("connected: %s", conn.RemoteAddr())
		return nil
	},
	OnData: func(conn *epoll.Conn, data []byte) error {
		return conn.Write(data)
	},
	OnClose: func(conn *epoll.Conn, cause error) {
		log.Printf("closed: %s (%v)", conn.RemoteAddr(), cause)
	},
}, epoll.WithMaxConnections(256))
if err != nil {
	return err
}

return srv.ListenAndServe(ctx)
```

The server supports TCP only; TLS is not available. `Address` uses `host:port` syntax. Port `0` asks the operating system to choose a port; after the server starts, use `srv.Addr()` to retrieve the bound address.

Runnable example: [`examples/epoll-server`](../examples/epoll-server). Start it with `go run ./examples/epoll-server -address 127.0.0.1:8082` on Linux.

## Callback behavior

- `OnConnect` runs after a connection is accepted and registered with epoll.
- `OnData` receives a copied, arbitrary chunk of the TCP byte stream. TCP message boundaries are not preserved; implement framing in the application.
- `OnClose` runs once when a connection is removed. Its error is nil for a normal peer close and non-nil for an I/O error, callback error, idle timeout, or server shutdown.
- A non-nil error from `OnConnect` or `OnData` closes that connection.

Callbacks execute synchronously and sequentially on the reactor goroutine. Keep them short and non-blocking so one callback does not delay other connections. `Conn.Write` copies bytes into an outgoing queue; the reactor handles partial writes and resumes when the socket becomes writable. `Conn.Close` schedules closure through the reactor. Both methods are safe to call from another goroutine.

## Limits and shutdown

`WithMaxConnections(limit)` sets the maximum number of accepted client connections. Its default is `min(GOMAXPROCS, 256)`; valid values are 1 through 4096. Connections beyond the limit are immediately closed.

`WithIdleTimeout(timeout)` closes connections without successful reads or writes after the given duration. The default is two minutes; the value must be positive.

`ListenAndServe` runs until its context is canceled, `Close` is called, or a listener/reactor error occurs. Use a signal-aware context for process shutdown. A server instance can only be started once.
