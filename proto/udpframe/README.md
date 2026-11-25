# udpframe

The `udpframe` package implements message framing for UDP connections. It splits messages into bounded datagrams, validates and reassembles fragments, and provides `DatagramConn`, which exposes request and response messages through the `net.Conn` stream interface.

## Wire format

Each datagram has a 28-byte header followed by a fragment payload. The diagram uses 32-bit rows; multi-row fields are contiguous and encoded in network byte order (big-endian):

```text
    0                   1                   2                   3
    0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |             Magic: "GONE" (bytes 0-3)                         |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |             Magic: "TUD1" (bytes 4-7)                         |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |             Message ID, most significant 32 bits              |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |             Message ID, least significant 32 bits             |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                 Fragment Index (32 bits)                     |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                 Fragment Count (32 bits)                     |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |              Total Message Size (32 bits)                    |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                 Fragment Payload (0-1172 bytes)               |
   |                              ...                              |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

The header fields occupy byte offsets 0 through 27; the payload starts at offset 28.

Each datagram has the following fields:

| Bytes | Field |
| --- | --- |
| `0–7` | Magic value `GONETUD1` |
| `8–15` | Non-zero message ID, big-endian `uint64` |
| `16–19` | Fragment index, zero-based big-endian `uint32` |
| `20–23` | Fragment count, big-endian `uint32` |
| `24–27` | Total message size, big-endian `uint32` |
| `28–1199` | Fragment payload |

Datagrams are at most 1200 bytes. With the 28-byte header, each fragment carries at most 1172 bytes. A message can be up to 16 MiB. Even an empty message is represented by one fragment. Receivers should match response fragments to the request ID and reassemble all fragments before processing the message.

## Framing messages

Use `NewID` to create a non-zero ID, then `WriteMessage` to frame and send a byte slice. Pass the same ID when sending the response so the requester can associate it with the request:

```go
id, err := udpframe.NewID()
if err != nil {
	return err
}
if err := udpframe.WriteMessage(udpConn, id, request); err != nil {
	return err
}
```

`WriteMessage` is suitable when the whole message is already in memory. `WriteMessageFrom` is useful when the payload comes from a reader: provide its exact byte size, and the function reads and frames that many bytes without buffering the entire message first.

```go
err := udpframe.WriteMessageFrom(udpConn, id, messageSize, source)
```

Both functions reject ID zero and sizes outside the supported range. A writer should write each supplied datagram as one datagram; a short write is reported as `io.ErrShortWrite`.

`Parse` validates one received datagram and returns its `Fragment` metadata and payload. Call it on each datagram read from the UDP socket. `FragmentCount` calculates how many datagrams a message requires; an empty message still takes one fragment.

Use one `Reassembler` per in-flight message. Call `Add` for each parsed fragment; it reports `complete == true` and returns the full payload when all fragments have arrived. It ignores fragments with a different ID, and ignores duplicate fragment indexes. The caller is responsible for limiting the number and lifetime of in-flight reassemblers.

For an unconnected UDP listener, `WriteMessage` can send to a selected peer through a small `io.Writer` adapter:

```go
type peerWriter struct {
	conn net.PacketConn
	addr net.Addr
}

func (w peerWriter) Write(data []byte) (int, error) {
	return w.conn.WriteTo(data, w.addr)
}
```

```go
var reassembler udpframe.Reassembler // use one reassembler per in-flight peer/message ID
for {
	datagram := make([]byte, udpframe.DatagramSize)
	n, peer, err := udpConn.ReadFrom(datagram)
	if err != nil {
		return err
	}

	fragment, err := udpframe.Parse(datagram[:n])
	if err != nil {
		continue // reject malformed datagrams
	}
	message, complete, err := reassembler.Add(fragment)
	if err != nil {
		return err
	}
	if !complete {
		continue
	}

	// Echo the completed payload; an application can transform it before replying.
	if err := udpframe.WriteMessage(peerWriter{conn: udpConn, addr: peer}, fragment.ID, message); err != nil {
		return err
	}
	reassembler = udpframe.Reassembler{} // start collecting the next message
}
```

The loop illustrates one message at a time. A concurrent UDP server must keep a separate reassembler per in-flight peer and message ID. `WriteMessage` accepts an `io.Writer`; the `peerWriter` adapter sends each framed fragment to the selected peer with `WriteTo`.

`NewID` generates a non-zero ID for pairing a request with its response.

## `DatagramConn`

Wrap a connected UDP `net.Conn` with `NewDatagramConn` when you want the UDP message exchange to look like a stream. `Write` buffers request bytes; `Read` sends the request if needed and waits for response fragments with the matching ID. The application protocol still needs to define how the receiver knows the response length. This example uses a four-byte big-endian length prefix:

```go
conn := udpframe.NewDatagramConn(udpConn)
conn.BeginCall()

if _, err := conn.Write(request); err != nil {
	return err
}
if err := conn.FlushRequest(); err != nil { // optional; Read also flushes lazily
	return err
}
var size [4]byte
if _, err := io.ReadFull(conn, size[:]); err != nil {
	return err
}
response := make([]byte, binary.BigEndian.Uint32(size[:]))
if _, err := io.ReadFull(conn, response); err != nil {
	return err
}
```

Call `BeginCall` before each request when using the adapter directly, including before the first request. Call `FlushRequest` only if the request should be sent before the first read; otherwise the first non-empty `Read` flushes it automatically. `RequestStarted` tells whether the current request has any written data, which can be useful to avoid sending an empty request when the handler produced no request. After reading the full response, call `BeginCall` to reset the adapter for the next exchange. An empty response is exposed as `io.EOF`.

The adapter is intended for one request at a time per connection. The peer must implement this framing protocol and echo the request ID in its response. Consume the complete response before reusing the connection. Requests larger than 16 MiB fail; on such an error, discard the connection or start a new call only after resetting the adapter with `BeginCall`.
