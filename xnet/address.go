/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package xnet

import (
	"fmt"
	"net"
	"net/netip"
	"strings"

	"go.osspkg.com/errors"
)

var (
	ErrResolveTCPAddress = errors.New("resolve tcp address")
)

// RandomPort returns an OS-assigned TCP address after releasing its listener.
// Use ListenRandomPort when the port must remain reserved.
func RandomPort(host string) (string, error) {
	l, err := ListenRandomPort(host)
	if err != nil {
		return net.JoinHostPort(host, "0"), err
	}

	address := l.Addr().String()
	if err = l.Close(); err != nil {
		return net.JoinHostPort(host, "0"), errors.Wrap(err, ErrResolveTCPAddress)
	}

	return address, nil
}

func ListenRandomPort(host string) (*net.TCPListener, error) {
	network := "tcp4"
	if strings.Contains(host, ":") {
		network = "tcp6"
	}

	address := net.JoinHostPort(host, "0")
	addr, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, errors.Wrap(err, ErrResolveTCPAddress)
	}

	l, err := net.ListenTCP(network, addr)
	if err != nil {
		return nil, errors.Wrap(err, ErrResolveTCPAddress)
	}

	return l, nil
}

func ResolveIPPort(address string) (string, error) {
	if strings.Contains(address, "/") {
		return address, nil
	}

	var (
		host string
		port string
	)

	switch {
	case address == "":
		host = "127.0.0.1"

	case address[0] == '[':
		var err error
		host, port, err = net.SplitHostPort(address)
		if err != nil {
			if strings.HasSuffix(address, "]") && IsValidIP(address[1:len(address)-1]) {
				host = address[1 : len(address)-1]
				break
			}
			return "", errors.Wrapf(err, "invalid address %q", address)
		}

	case IsValidIP(address):
		host = address

	case strings.Count(address, ":") > 1:
		return "", fmt.Errorf("invalid address %q: IPv6 addresses with a port must be bracketed", address)

	case strings.Contains(address, ":"):
		var err error
		host, port, err = net.SplitHostPort(address)
		if err != nil {
			return "", errors.Wrapf(err, "invalid address %q", address)
		}

	default:
		host = address
	}

	if port != "" {
		if _, err := net.LookupPort("tcp", port); err != nil {
			return "", errors.Wrapf(err, "invalid tcp port %q", port)
		}
	}

	if host == "" {
		host = "0.0.0.0"
	}

	if !IsValidIP(host) {
		if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
			host = ips[0].String()
		}
	}

	if port == "" {
		port = "8080"
	}

	return net.JoinHostPort(host, port), nil
}

func FixIPPort(defaultPort string, ips ...string) []string {
	result := make([]string, 0, len(ips))
	for _, ip := range ips {
		host, port, err := net.SplitHostPort(ip)
		if err != nil {
			host = ip
			port = defaultPort
		}

		if !IsValidIP(host) {
			continue
		}

		if port == "" || port == "0" {
			port = defaultPort
		}

		result = append(result, net.JoinHostPort(host, port))
	}

	return result
}

func IsValidIP(ip string) bool {
	_, err := netip.ParseAddr(ip)
	return err == nil
}

func SplitAddressType(addresses []string) (ips []net.IP, domains []string) {
	for _, address := range addresses {
		host := address
		if san, _, err := net.SplitHostPort(address); err == nil {
			host = san
		} else if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
		}

		if ip := net.ParseIP(host); ip != nil {
			ips = append(ips, ip)
		} else {
			domains = append(domains, host)
		}
	}
	return
}
