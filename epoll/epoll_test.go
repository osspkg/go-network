/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package epoll

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		conf Config
		opts []Option
	}{
		{name: "empty address", conf: Config{OnData: func(*Conn, []byte) error { return nil }}},
		{name: "invalid address", conf: Config{Address: "localhost", OnData: func(*Conn, []byte) error { return nil }}},
		{name: "missing callback", conf: Config{Address: "127.0.0.1:8080"}},
		{name: "nil option", conf: Config{Address: "127.0.0.1:8080", OnData: func(*Conn, []byte) error { return nil }}, opts: []Option{nil}},
		{name: "zero connection limit", conf: Config{Address: "127.0.0.1:8080", OnData: func(*Conn, []byte) error { return nil }}, opts: []Option{WithMaxConnections(0)}},
		{name: "too many connections", conf: Config{Address: "127.0.0.1:8080", OnData: func(*Conn, []byte) error { return nil }}, opts: []Option{WithMaxConnections(4097)}},
		{name: "zero idle timeout", conf: Config{Address: "127.0.0.1:8080", OnData: func(*Conn, []byte) error { return nil }}, opts: []Option{WithIdleTimeout(0)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := New(tc.conf, tc.opts...); got != nil || err == nil {
				t.Fatalf("New() = (%v, %v), want validation error", got, err)
			}
		})
	}
}

func TestNewAndLifecycleValidation(t *testing.T) {
	srv, err := New(Config{Address: "127.0.0.1:0", OnData: func(*Conn, []byte) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if srv.Addr() != nil {
		t.Fatalf("Addr before start = %v, want nil", srv.Addr())
	}
	if err := srv.ListenAndServe(nil); err == nil {
		t.Fatal("ListenAndServe(nil) succeeded")
	}
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	if err := srv.ListenAndServe(context.Background()); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("ListenAndServe after Close() = %v, want closed error", err)
	}
}

func TestOptions(t *testing.T) {
	for _, option := range []Option{WithMaxConnections(1), WithMaxConnections(4096), WithIdleTimeout(time.Millisecond)} {
		if _, err := New(Config{Address: "127.0.0.1:8080", OnData: func(*Conn, []byte) error { return nil }}, option); err != nil {
			t.Fatalf("New with valid option: %v", err)
		}
	}
}
