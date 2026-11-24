/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"go.osspkg.com/network/cert"
	"go.osspkg.com/network/proto/udpframe"
)

func TestNewValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		conf Config
		opts []Option
	}{
		{name: "unsupported network", conf: Config{Network: "sctp", Address: "127.0.0.1:1"}},
		{name: "empty address", conf: Config{Network: "tcp"}},
		{name: "invalid TCP address", conf: Config{Network: "tcp", Address: "localhost"}},
		{name: "invalid UDP address", conf: Config{Network: "udp", Address: "localhost"}},
		{name: "invalid Unix address", conf: Config{Network: "unix"}},
		{name: "nil option", conf: Config{Network: "tcp", Address: "127.0.0.1:1"}, opts: []Option{nil}},
		{name: "invalid max connections", conf: Config{Network: "tcp", Address: "127.0.0.1:1"}, opts: []Option{WithMaxConnections(0)}},
		{name: "invalid idle timeout", conf: Config{Network: "tcp", Address: "127.0.0.1:1"}, opts: []Option{WithIdleTimeout(0)}},
		{name: "TLS on UDP", conf: Config{Network: "udp", Address: "127.0.0.1:1", Certificate: &cert.ServerConfig{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := New(tc.conf, tc.opts...); got != nil || err == nil {
				t.Fatalf("New() = (%v, %v), want error", got, err)
			}
		})
	}
}

func TestNewAndCloseLifecycle(t *testing.T) {
	srv, err := New(Config{Network: "tcp", Address: "127.0.0.1:0"}, WithMaxConnections(2), WithIdleTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.ListenAndServe(context.Background()); err == nil {
		t.Fatal("ListenAndServe without handler succeeded")
	}
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("second Close() returned error: %v", err)
	}
	if err := srv.ListenAndServe(context.Background()); err == nil {
		t.Fatal("ListenAndServe after Close succeeded")
	}
}

func TestListenAndServeRejectsNilContext(t *testing.T) {
	srv, err := New(Config{Network: "tcp", Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	srv.HandleFunc(func(context.Context, io.ReadWriter, net.Addr) error { return nil })
	if err := srv.ListenAndServe(nil); err == nil {
		t.Fatal("ListenAndServe(nil) succeeded")
	}
}

func TestDatagramHandlerRoundTrip(t *testing.T) {
	peer := testUDPAddr("client")
	conn := &testPacketConn{}
	request := []byte("udp request")
	server := &object{handler: func(_ context.Context, rw io.ReadWriter, addr net.Addr) error {
		if addr.String() != peer.String() {
			t.Errorf("handler addr = %q, want %q", addr, peer)
		}
		got, err := io.ReadAll(rw)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, request) {
			t.Errorf("handler request = %q, want %q", got, request)
		}
		_, err = rw.Write([]byte("udp response"))
		return err
	}}
	var frames packetCapture
	if err := udpframe.WriteMessage(&frames, 42, request); err != nil {
		t.Fatal(err)
	}
	fragment, err := udpframe.Parse(frames.packets[0])
	if err != nil {
		t.Fatal(err)
	}
	server.handleDatagram(context.Background(), conn, udpRequest{addr: peer, id: 42, data: fragment.Payload}, make(chan struct{}, 16))
	if len(conn.writes) != 1 {
		t.Fatalf("WriteTo calls = %d, want 1", len(conn.writes))
	}
	if conn.addresses[0].String() != peer.String() {
		t.Fatalf("response address = %q, want %q", conn.addresses[0], peer)
	}
	response, err := udpframe.Parse(conn.writes[0])
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != 42 || string(response.Payload) != "udp response" {
		t.Fatalf("unexpected response fragment: %+v", response)
	}
}

func TestDatagramHandlerDoesNotReplyOnHandlerError(t *testing.T) {
	conn := &testPacketConn{}
	srv := &object{handler: func(context.Context, io.ReadWriter, net.Addr) error { return errors.New("handler failure") }}
	srv.handleDatagram(context.Background(), conn, udpRequest{addr: testUDPAddr("peer"), id: 3, data: []byte("request")}, make(chan struct{}, 1))
	if len(conn.writes) != 0 {
		t.Fatalf("writes = %d, want no reply after handler error", len(conn.writes))
	}
}

func TestHandleStreamRunsHandlerAndReleasesConnection(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close() //nolint:errcheck
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := &object{
		handler: func(_ context.Context, rw io.ReadWriter, _ net.Addr) error {
			request := make([]byte, 4)
			if _, err := io.ReadFull(rw, request); err != nil {
				return err
			}
			_, err := rw.Write(request)
			return err
		},
		active: make(map[net.Conn]struct{}),
	}
	slots := make(chan struct{}, 1)
	slots <- struct{}{}
	if !srv.track(serverConn) {
		t.Fatal("track() rejected connection")
	}
	srv.workers.Add(1)
	go srv.handleStream(ctx, serverConn, slots)

	if _, err := clientConn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if _, err := io.ReadFull(clientConn, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != "ping" {
		t.Fatalf("response = %q, want ping", response)
	}
	_ = clientConn.Close()
	srv.workers.Wait()
	if len(srv.active) != 0 {
		t.Fatalf("active connections = %d, want 0 after handler exits", len(srv.active))
	}
	if len(slots) != 0 {
		t.Fatalf("connection slot was not released: %d remain", len(slots))
	}
}

func TestUDPResponseReadWrite(t *testing.T) {
	response := udpResponse{budget: make(chan struct{}, 2)}
	first, err := response.Write([]byte("hello "))
	if err != nil || first != 6 {
		t.Fatalf("Write() = (%d, %v)", first, err)
	}
	if _, err := response.Write([]byte("world")); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(&response)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Fatalf("Read() = %q, want hello world", got)
	}
	if len(response.budget) != response.units {
		t.Fatalf("reserved units = %d, want %d", len(response.budget), response.units)
	}
	releaseBudget(response.budget, response.units)
	if len(response.budget) != 0 {
		t.Fatalf("budget not released: %d units remain", len(response.budget))
	}
}

func TestReserveBudgetIsAtomic(t *testing.T) {
	budget := make(chan struct{}, 2)
	if !reserveBudget(budget, 2) {
		t.Fatal("reserveBudget failed with available capacity")
	}
	if reserveBudget(budget, 1) {
		t.Fatal("reserveBudget succeeded beyond capacity")
	}
	if len(budget) != 2 {
		t.Fatalf("failed reservation leaked tokens: got %d, want 2", len(budget))
	}
	releaseBudget(budget, 2)
}

type packetCapture struct{ packets [][]byte }

func (p *packetCapture) Write(data []byte) (int, error) {
	p.packets = append(p.packets, append([]byte(nil), data...))
	return len(data), nil
}

type testPacketConn struct {
	writes    [][]byte
	addresses []net.Addr
}

func (c *testPacketConn) ReadFrom([]byte) (int, net.Addr, error) { return 0, nil, io.EOF }
func (c *testPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	c.writes = append(c.writes, append([]byte(nil), p...))
	c.addresses = append(c.addresses, addr)
	return len(p), nil
}
func (*testPacketConn) Close() error                     { return nil }
func (*testPacketConn) LocalAddr() net.Addr              { return testUDPAddr("server") }
func (*testPacketConn) SetDeadline(time.Time) error      { return nil }
func (*testPacketConn) SetReadDeadline(time.Time) error  { return nil }
func (*testPacketConn) SetWriteDeadline(time.Time) error { return nil }

type testUDPAddr string

func (a testUDPAddr) Network() string { return "udp" }
func (a testUDPAddr) String() string  { return string(a) }

var _ net.PacketConn = (*testPacketConn)(nil)
