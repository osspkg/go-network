/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package listen

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRejectsInvalidNetwork(t *testing.T) {
	got, err := New(context.Background(), "sctp", "127.0.0.1:0", nil)
	if got != nil || err == nil || !strings.Contains(err.Error(), "invalid network type") {
		t.Fatalf("New() = (%v, %v), want invalid network error", got, err)
	}
}

func TestNewRejectsInvalidAddresses(t *testing.T) {
	for _, tc := range []struct {
		network string
		address string
	}{
		{network: "tcp", address: "missing-port"},
		{network: "udp", address: "missing-port"},
		{network: "unix", address: ""},
	} {
		t.Run(tc.network, func(t *testing.T) {
			if got, err := New(context.Background(), tc.network, tc.address, nil); got != nil || err == nil {
				t.Fatalf("New(%q, %q) = (%v, %v), want error", tc.network, tc.address, got, err)
			}
		})
	}
}

func TestNewCreatesTCPAndPacketListeners(t *testing.T) {
	tcp, err := New(context.Background(), "tcp", "127.0.0.1:0", nil)
	if err != nil {
		skipIfSocketUnavailable(t, err)
		t.Fatal(err)
	}
	listener, ok := tcp.(net.Listener)
	if !ok {
		t.Fatalf("TCP listener type = %T, want net.Listener", tcp)
	}
	if listener.Addr() == nil {
		t.Fatal("TCP listener has nil address")
	}
	if err := tcp.Close(); err != nil {
		t.Fatal(err)
	}

	packet, err := New(context.Background(), "udp", "127.0.0.1:0", nil)
	if err != nil {
		skipIfSocketUnavailable(t, err)
		t.Fatal(err)
	}
	packetConn, ok := packet.(net.PacketConn)
	if !ok {
		t.Fatalf("UDP listener type = %T, want net.PacketConn", packet)
	}
	if packetConn.LocalAddr() == nil {
		t.Fatal("UDP packet listener has nil local address")
	}
	if err := packet.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewCreatesUnixListener(t *testing.T) {
	address := filepath.Join(t.TempDir(), "listener.sock")
	closer, err := New(context.Background(), "unix", address, nil)
	if err != nil {
		skipIfSocketUnavailable(t, err)
		t.Fatal(err)
	}
	listener, ok := closer.(net.Listener)
	if !ok {
		t.Fatalf("Unix listener type = %T, want net.Listener", closer)
	}
	if listener.Addr() == nil {
		t.Fatal("Unix listener has nil address")
	}
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewTCPWithTLSReturnsListener(t *testing.T) {
	config := &tls.Config{MinVersion: tls.VersionTLS13}
	closer, err := New(context.Background(), "tcp", "127.0.0.1:0", config)
	if err != nil {
		skipIfSocketUnavailable(t, err)
		t.Fatal(err)
	}
	listener, ok := closer.(net.Listener)
	if !ok {
		t.Fatalf("TLS listener type = %T, want net.Listener", closer)
	}
	if listener.Addr() == nil {
		t.Fatal("TLS listener has nil address")
	}
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
}

func skipIfSocketUnavailable(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, os.ErrPermission) || strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {
		t.Skipf("socket operations are restricted in this environment: %v", err)
	}
}
