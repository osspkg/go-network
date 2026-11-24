# Package selection and shared contracts

The module is `go.osspkg.com/network` and its `go.mod` targets Go 1.26. Choose the package based on the layer being implemented:

| Need | Package |
| --- | --- |
| Concurrent request calls over pooled connections | `client` |
| Callback server for TCP, UDP, or Unix streams | `server` |
| Linux TCP readiness callbacks on one epoll reactor | `epoll` |
| Create a listener without a server abstraction | `listen` |
| TLS config and certificate loading/generation | `cert` |
| UDP frame parsing, fragmentation, or message adapter | `proto/udpframe` |
| Address normalization and transport names | `xnet` |
| Get an OS file descriptor from a supported value | `fd` |

`client.Config` and `server.Config` require explicit `Network` and `Address`; the struct tags document defaults for config decoding but Go struct literals are not defaulted. Supported network names are `tcp`, `udp`, and `unix`.

TCP and Unix are byte streams. Define message boundaries in the application protocol (for example, a fixed-size length prefix) and read the complete response before returning a successful pooled client call. UDP handlers receive one complete `GONETUD1` request message and write one response message; the protocol fragments it into datagrams but does not retransmit lost packets.

The `server` package starts listening inside `ListenAndServe`. Register `HandleFunc` first, then pass a cancelable context. `Close` stops the server; a server instance cannot be restarted. `client.Client` supports concurrent `Call` invocations and should be closed when no longer needed.

`epoll` is for Linux TCP only. Its callbacks run synchronously on the event-loop goroutine and must not block. It emits arbitrary byte chunks, not complete messages. Use `Conn.Write` to queue output.
