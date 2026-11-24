# fd

The `fd` package retrieves an operating system file descriptor from supported Go connection and file values. It returns the descriptor as an `int64`, or `-1` when the value is nil or does not expose a descriptor.

## Getting a descriptor from a connection

`ByConnect` accepts a `net.Conn` and retrieves its descriptor when the connection implements `syscall.Conn`. `BySyscallConn` accepts `syscall.Conn` directly and uses its `SyscallConn` method:

```go
descriptor := fd.ByConnect(conn)
if descriptor < 0 {
	return errors.New("connection does not expose a file descriptor")
}
```

Some connection implementations, such as `net.Pipe`, do not expose an operating system descriptor. Errors from `SyscallConn` or its `RawConn.Control` operation also result in `-1`.

## Getting a descriptor from a file

`ByFile` returns the descriptor owned by an `*os.File`:

```go
descriptor := fd.ByFile(file)
if descriptor < 0 {
	return errors.New("file is nil or closed")
}
```

The returned descriptor is only valid while its connection or file remains open. The package does not duplicate or take ownership of the descriptor. Callers must coordinate descriptor use with closing the underlying value.
