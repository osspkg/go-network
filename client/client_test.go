/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"go.osspkg.com/network/cert"
)

func TestNewValidation(t *testing.T) {
	tests := []struct {
		name string
		conf Config
		opts []Option
	}{
		{name: "unsupported network", conf: Config{Address: "127.0.0.1:1", Network: "ip"}},
		{name: "empty address", conf: Config{Network: "tcp"}},
		{name: "invalid TCP address", conf: Config{Address: "localhost", Network: "tcp"}},
		{name: "invalid UDP address", conf: Config{Address: "localhost", Network: "udp"}},
		{name: "invalid Unix address", conf: Config{Network: "unix"}},
		{name: "nil option", conf: Config{Address: "127.0.0.1:1", Network: "tcp"}, opts: []Option{nil}},
		{name: "zero pool size", conf: Config{Address: "127.0.0.1:1", Network: "tcp"}, opts: []Option{WithPoolSize(0)}},
		{name: "negative idle timeout", conf: Config{Address: "127.0.0.1:1", Network: "tcp"}, opts: []Option{WithIdleTimeout(-time.Second)}},
		{name: "TLS on UDP", conf: Config{Address: "127.0.0.1:1", Network: "udp", Certificate: &cert.ClientConfig{}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := New(tc.conf, tc.opts...); got != nil || err == nil {
				t.Fatalf("New() = (%v, %v), want error", got, err)
			}
		})
	}
}

func TestCallValidationAndClosedClient(t *testing.T) {
	cli, _ := newPipeClient(t, 1, true)
	ctx := context.Background()
	if err := cli.Call(nil, func(io.ReadWriter) error { return nil }); err == nil {
		t.Fatal("Call(nil context) succeeded")
	}
	if err := cli.Call(ctx, nil); err == nil {
		t.Fatal("Call(nil handler) succeeded")
	}
	if err := cli.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cli.Close(); err != nil {
		t.Fatalf("second Close() returned error: %v", err)
	}
	if err := cli.Call(ctx, func(io.ReadWriter) error { return nil }); !errors.Is(err, ErrClientClosed) {
		t.Fatalf("Call() after Close() error = %v, want ErrClientClosed", err)
	}
}

func TestCallReusesConnection(t *testing.T) {
	cli, dials := newPipeClient(t, 1, true)
	for i := range 3 {
		if err := echoCall(cli, context.Background(), []byte{byte('a' + i)}); err != nil {
			t.Fatalf("Call %d: %v", i, err)
		}
	}
	if got := dials.Load(); got != 1 {
		t.Fatalf("dial count = %d, want 1 reused connection", got)
	}
}

func TestCallDiscardsConnectionAfterHandlerError(t *testing.T) {
	cli, dials := newPipeClient(t, 1, true)
	wantErr := errors.New("handler failed")
	if err := cli.Call(context.Background(), func(io.ReadWriter) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("Call() error = %v, want handler error", err)
	}
	if err := echoCall(cli, context.Background(), []byte("next")); err != nil {
		t.Fatal(err)
	}
	if got := dials.Load(); got != 2 {
		t.Fatalf("dial count = %d, want 2 after failed call discards connection", got)
	}
}

func TestCallContextCancellationClosesConnection(t *testing.T) {
	cli, _ := newPipeClient(t, 1, false)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	entered := make(chan struct{})
	go func() {
		done <- cli.Call(ctx, func(rw io.ReadWriter) error {
			close(entered)
			_, err := rw.Read(make([]byte, 1))
			return err
		})
	}()
	<-entered
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Call() succeeded after context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("Call() did not stop after context cancellation")
	}
}

func TestCallWaitsForPoolSlotAndHonorsContext(t *testing.T) {
	cli, dials := newPipeClient(t, 1, false)
	entered := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- cli.Call(context.Background(), func(io.ReadWriter) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := cli.Call(ctx, func(io.ReadWriter) error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting Call() error = %v, want deadline exceeded", err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Call() error: %v", err)
	}
	if got := dials.Load(); got != 1 {
		t.Fatalf("dial count = %d, want 1", got)
	}
}

func TestOptionsValidation(t *testing.T) {
	for _, option := range []Option{WithPoolSize(-1), WithPoolSize(0), WithIdleTimeout(0), WithIdleTimeout(-time.Second)} {
		opts := defaultOptions()
		if err := option(&opts); err == nil {
			t.Fatalf("option %T accepted invalid value", option)
		}
	}
}

func newPipeClient(t *testing.T, size int, echo bool) (Client, *atomic.Int32) {
	t.Helper()
	var dials atomic.Int32
	cli := &object{}
	cli.pool = newConnPool(size, time.Minute, func(context.Context) (net.Conn, error) {
		dials.Add(1)
		clientConn, peerConn := net.Pipe()
		if echo {
			go func() {
				defer peerConn.Close() //nolint:errcheck
				echoProtocol(peerConn)
			}()
		} else {
			t.Cleanup(func() { _ = peerConn.Close() })
		}
		return clientConn, nil
	})
	t.Cleanup(func() { _ = cli.Close() })
	return cli, &dials
}

func echoProtocol(conn net.Conn) {
	for {
		var size [4]byte
		if _, err := io.ReadFull(conn, size[:]); err != nil {
			return
		}
		payload := make([]byte, binary.BigEndian.Uint32(size[:]))
		if _, err := io.ReadFull(conn, payload); err != nil {
			return
		}
		if _, err := conn.Write(size[:]); err != nil {
			return
		}
		if _, err := conn.Write(payload); err != nil {
			return
		}
	}
}

func echoCall(cli Client, ctx context.Context, payload []byte) error {
	return cli.Call(ctx, func(rw io.ReadWriter) error {
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(payload)))
		if _, err := rw.Write(size[:]); err != nil {
			return err
		}
		if _, err := rw.Write(payload); err != nil {
			return err
		}
		if _, err := io.ReadFull(rw, size[:]); err != nil {
			return err
		}
		response := make([]byte, binary.BigEndian.Uint32(size[:]))
		if _, err := io.ReadFull(rw, response); err != nil {
			return err
		}
		if string(response) != string(payload) {
			return errors.New("echo response differs from request")
		}
		return nil
	})
}
