---
name: osspkg-go-network
description: Use go.osspkg.com/network APIs to build Go TCP, UDP, Unix-socket clients and servers, configure TLS, frame UDP messages, or use the module's network helpers.
---

# Osspkg Go Network

Use this skill when writing or changing Go code that imports `go.osspkg.com/network`.

Start by identifying the transport and interaction model, then read only the relevant local reference:

- [Package selection and shared contracts](references/packages.md) for choosing packages and lifecycle behavior.
- [Client and server examples](references/client-server.md) for pooled clients, stream handlers, and framed UDP handlers.
- [UDP framing](references/udpframe.md) when implementing or debugging `GONETUD1` messages.
- [TLS and utility packages](references/tls-and-utilities.md) for certificates, listeners, file descriptors, and address helpers.

Treat the references as usage guidance, not as a substitute for current source. Before relying on a detail that affects correctness, inspect the package's current exported API and implementation in the consuming checkout. Keep TCP/Unix stream framing in the application protocol; UDP uses the module's message framing. `epoll` is a separate Linux-only TCP event API and is not interchangeable with the stream `server` handler contract.

All supporting links and code references are contained in this skill directory.
