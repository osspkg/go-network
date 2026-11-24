# xnet

The `xnet` package provides small helpers for validating supported network names, normalizing host and port addresses, reserving TCP ports, and separating IP addresses from DNS names.

## Supported network names

Use the package constants when configuring a network transport:

```go
switch network {
case xnet.NetTCP, xnet.NetUDP, xnet.NetUNIX:
	// supported by this module
default:
	return xnet.IsPassableNetwork(network)
}
```

`IsPassableNetwork` returns nil for `NetTCP` (`"tcp"`), `NetUDP` (`"udp"`), and `NetUNIX` (`"unix"`), and an error for other values.

## Normalize an IP or host address

`ResolveIPPort` converts an IP address, hostname, or host-and-port string into a `host:port` address. It uses `127.0.0.1` when the input is empty and port `8080` when no port is supplied. A missing host with an explicit port becomes `0.0.0.0`:

```go
address, err := xnet.ResolveIPPort("192.0.2.10")
if err != nil {
	return err
}
// address is "192.0.2.10:8080"
```

IPv6 addresses with ports must be bracketed, for example `"[2001:db8::1]:443"`. IPv6 addresses without ports are accepted. Hostnames are resolved when DNS lookup succeeds; if it does not, the hostname is retained in the returned address. Inputs containing `/` are returned unchanged, which is useful for path-like addresses.

## Reserve a TCP port

Use `ListenRandomPort` when the selected TCP port must remain bound while you prepare to use it:

```go
listener, err := xnet.ListenRandomPort("127.0.0.1")
if err != nil {
	return err
}
defer listener.Close()

address := listener.Addr().String()
```

The returned `*net.TCPListener` keeps the OS-assigned port reserved until it is closed. Use `RandomPort` only when you need the address string and can tolerate the port being released before you bind to it; that gap allows another process to claim the port.

## Normalize a list of IP addresses

`FixIPPort` keeps valid IP addresses, adds the supplied default port where an address has no port or uses port `0`, and drops entries that are not IP addresses:

```go
addresses := xnet.FixIPPort("8080", "192.0.2.10", "[2001:db8::1]:0", "example.test")
// []string{"192.0.2.10:8080", "[2001:db8::1]:8080"}
```

## Split IPs and DNS names

`SplitAddressType` classifies address strings as IPs or non-IP host strings. It accepts plain hosts, bracketed IPv6 addresses, and `host:port` values. Any host that is not parsed as an IP is placed in the second result; the function does not validate DNS syntax or resolve the name:

```go
ips, names := xnet.SplitAddressType([]string{
	"192.0.2.10:443",
	"[2001:db8::1]",
	"api.example.test:443",
})
// ips contains the two IP addresses; names contains "api.example.test".
```

## Validate a single IP

`IsValidIP` reports whether a string parses as an IPv4 or IPv6 address. It does not accept hostnames or host-and-port strings:

```go
if !xnet.IsValidIP("2001:db8::1") {
	return errors.New("invalid IP address")
}
```
