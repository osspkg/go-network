# go-network

`go.osspkg.com/network` is a Go networking module with reusable clients and servers for TCP, UDP, and Unix domain sockets. It includes connection pooling, optional TLS, UDP message framing, and helpers for working with network addresses and file descriptors.

## Requirements

- Go 1.26 or later

## Installation

```sh
go get go.osspkg.com/network/client
```

Import the packages you need, for example:

```go
import (
	"go.osspkg.com/network/client"
	"go.osspkg.com/network/server"
)
```

## Packages

| Package | Description |
| --- | --- |
| [`client`](client/README.md) | Concurrent request handling over a bounded pool of reusable connections. Supports TCP, UDP, and Unix sockets. |
| [`server`](server/README.md) | Handler-based TCP, UDP, and Unix servers with configurable connection limits and idle timeouts. |
| [`epoll`](epoll/README.md) | Linux epoll-driven TCP server with ordered event callbacks and queued writes. |
| [`cert`](cert/README.md) | TLS configuration, certificate loading, and CA/server/client certificate generation. |
| [`listen`](listen/README.md) | TCP, UDP, and Unix listener creation, with optional TCP TLS wrapping. |
| [`proto/udpframe`](proto/udpframe/README.md) | UDP message framing, fragmentation, parsing, and reassembly. |
| [`xnet`](xnet/README.md) | Address normalization, IP helpers, and supported network constants. |
| [`fd`](fd/README.md) | File descriptor access for supported connections and `os.File` values. |

## Examples

Runnable examples are available in [`examples/client`](examples/client), [`examples/server`](examples/server), and [`examples/epoll-server`](examples/epoll-server). The regular server example implements a length-prefixed echo protocol over TCP, Unix sockets, or framed UDP. The epoll example is a Linux-only TCP byte-stream echo server.

For example, start a TCP echo server:

```sh
go run ./examples/server
```

Then run the client in another terminal:

```sh
go run ./examples/client -address 127.0.0.1:8080 -network tcp
```

On Linux, start the epoll example separately with `go run ./examples/epoll-server -address 127.0.0.1:8082`. It echoes raw TCP bytes and is not compatible with the length-prefixed client example.

See each example's `-h` output for available flags.

## Development

Run package tests with:

```sh
go test ./...
```

The repository also provides `make tests`, `make lint`, `make build`, and `make pre-commit` targets.

## License

This project is licensed under the BSD 3-Clause License. See [LICENSE](LICENSE).
