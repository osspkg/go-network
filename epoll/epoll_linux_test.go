//go:build linux

/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package epoll

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestEpollTCPRoundTripAndShutdown(t *testing.T) {
	connected := make(chan struct{}, 1)
	closed := make(chan error, 1)
	srv, err := New(Config{
		Address: "127.0.0.1:0",
		OnConnect: func(*Conn) error {
			connected <- struct{}{}
			return nil
		},
		OnData:  func(conn *Conn, data []byte) error { return conn.Write(data) },
		OnClose: func(_ *Conn, err error) { closed <- err },
	}, WithIdleTimeout(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.ListenAndServe(ctx) }()

	address := waitForAddress(t, srv, serveDone)
	client, err := net.DialTimeout("tcp", address.String(), time.Second)
	if err != nil {
		skipIfNetworkDenied(t, err)
		t.Fatal(err)
	}
	defer client.Close() //nolint:errcheck
	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("OnConnect callback was not called")
	}

	request := []byte("epoll round trip")
	if _, err := client.Write(request); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, len(request))
	if _, err := io.ReadFull(client, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != string(request) {
		t.Fatalf("response = %q, want %q", response, request)
	}
	_ = client.Close()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("OnClose callback was not called")
	}

	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("ListenAndServe() = %v, want nil after Close", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reactor did not stop after Close")
	}
}

func waitForAddress(t *testing.T, srv Server, serveDone <-chan error) net.Addr {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if address := srv.Addr(); address != nil {
			return address
		}
		select {
		case err := <-serveDone:
			skipIfNetworkDenied(t, err)
			t.Fatalf("ListenAndServe failed before binding: %v", err)
		case <-deadline:
			t.Fatal("server did not bind within timeout")
		case <-time.After(time.Millisecond):
		}
	}
}

func skipIfNetworkDenied(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, net.ErrClosed) || strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {
		t.Skipf("network operations are restricted in this environment: %v", err)
	}
}

func TestConnWriteAndCloseState(t *testing.T) {
	conn := &Conn{server: &server{wake: -1}, fd: -1}
	if err := conn.Write([]byte("queued")); err != nil {
		t.Fatal(err)
	}
	if string(conn.out) != "queued" {
		t.Fatalf("queued bytes = %q", conn.out)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := conn.Write([]byte("after close")); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Write after Close() = %v, want net.ErrClosed", err)
	}
}
