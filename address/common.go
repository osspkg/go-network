/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package address

import (
	"net"
	"strings"

	"go.osspkg.com/errors"
)

var (
	ErrResolveTCPAddress = errors.New("resolve tcp address")
)

func RandomPort(host string) (string, error) {
	network := "tcp4"
	if strings.Contains(host, ":") {
		network = "tcp6"
	}

	host = net.JoinHostPort(host, "0")
	addr, err := net.ResolveTCPAddr(network, host)
	if err != nil {
		return host, errors.Wrap(err, ErrResolveTCPAddress)
	}

	l, err := net.ListenTCP(network, addr)
	if err != nil {
		return host, errors.Wrap(err, ErrResolveTCPAddress)
	}

	v := l.Addr().String()

	if err = l.Close(); err != nil {
		return host, errors.Wrap(err, ErrResolveTCPAddress)
	}

	return v, nil
}

func ResolveIPPort(address string) string {
	var (
		host string
		port string
	)

	switch true {
	case len(address) == 0:
		host = "127.0.0.1"

	case IsValidIP(address):
		host = address

	case address[0] == '[':
		if index := strings.IndexByte(address, ']'); index != -1 {
			host = address[1:index]
			port = address[index+1:]
			if len(port) > 1 && port[0] == ':' {
				port = port[1:]
			}
		}
		if !IsValidIP(host) {
			host = "::1"
		}

	case strings.Count(address, ":") > 1:
		host = address
		if !IsValidIP(host) {
			host = "::1"
		}

	case strings.Count(address, ":") == 1:
		index := strings.IndexByte(address, ':')
		host = address[0:index]
		port = address[index+1:]
		if len(port) > 1 && port[0] == ':' {
			port = port[1:]
		}

	default:
		host = address
	}

	if strings.Contains(host, "/") {
		return host
	}

	if len(host) == 0 {
		host = "0.0.0.0"
	}

	if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
		host = ips[0].String()
	}

	if len(port) == 0 || port == ":" {
		if v, err := RandomPort(host); err == nil {
			return v
		}
		port = "8080"
	}

	return net.JoinHostPort(host, port)
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

		if port == "0" {
			port = defaultPort
		}

		result = append(result, net.JoinHostPort(host, port))
	}

	return result
}

func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}
