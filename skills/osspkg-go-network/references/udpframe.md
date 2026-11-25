# UDP framing (`proto/udpframe`)

The `GONETUD1` frame has a 28-byte header and up to 1172 payload bytes per datagram. Datagram size is capped at 1200 bytes; messages are capped at 16 MiB. The header contains the magic, non-zero message ID, zero-based fragment index, fragment count, and total message size. Integers use big-endian encoding.

## Send and reassemble a message

`NewID` creates a request ID. `WriteMessage` frames a byte slice; `WriteMessageFrom` frames exactly a given number of bytes read from an `io.Reader` without first holding the entire message in memory.

```go
id, err := udpframe.NewID()
if err != nil {
	return err
}
if err := udpframe.WriteMessage(connectedUDP, id, payload); err != nil {
	return err
}
```

For each received datagram, call `Parse` before `Reassembler.Add`. Keep separate reassemblers for concurrent message IDs (and peers for an unconnected socket), bound their lifetime/count, and only process a message after `complete` is true:

```go
fragment, err := udpframe.Parse(datagram[:n])
if err != nil {
	return err // reject malformed frames
}
message, complete, err := assembler.Add(fragment)
if err != nil {
	return err
}
if complete {
	// Process message and reply using fragment.ID.
}
```

For an unconnected `net.PacketConn`, adapt `WriteTo` to an `io.Writer` so `WriteMessage` sends each generated frame to the selected peer. The response must use the request ID. The wire protocol does not provide retransmission or ordering recovery beyond fragment reassembly.

## Stream-shaped UDP client connection

`NewDatagramConn` wraps a connected UDP `net.Conn` as an `io.ReadWriter`. Call `BeginCall` before each direct exchange, write the complete request, then read the application-defined response. The first non-empty read flushes the request lazily; `FlushRequest` can send it earlier. An empty message is exposed as EOF. The peer must return response fragments with the matching request ID.

```go
conn := udpframe.NewDatagramConn(udpConn)
conn.BeginCall()
if _, err := conn.Write(request); err != nil {
	return err
}

// The application protocol defines the response length. This example uses
// a four-byte big-endian prefix.
var header [4]byte
if _, err := io.ReadFull(conn, header[:]); err != nil {
	return err
}
response := make([]byte, binary.BigEndian.Uint32(header[:]))
_, err := io.ReadFull(conn, response)
return err
```

When called through `client.Client`, the client resets the adapter for each call and flushes a written request after the handler returns if no read flushed it first. Consume the response according to the peer's application protocol.
