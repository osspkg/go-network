//go:build !linux

/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package epoll

import (
	"context"
	"net"

	"go.osspkg.com/errors"
)

// Conn is an unavailable connection placeholder on non-Linux platforms.
type Conn struct{}

func (*Conn) Write([]byte) error   { return errors.New("epoll is only supported on Linux") }
func (*Conn) Close() error         { return errors.New("epoll is only supported on Linux") }
func (*Conn) RemoteAddr() net.Addr { return nil }

type unsupportedServer struct{}

// New validates configuration but epoll cannot be constructed off Linux.
func New(c Config, opts ...Option) (Server, error) {
	if _, err := validate(c, opts); err != nil {
		return nil, err
	}
	if c.OnData == nil {
		return nil, errors.New("epoll OnData callback is required")
	}
	return unsupportedServer{}, nil
}

func (unsupportedServer) ListenAndServe(context.Context) error {
	return errors.New("epoll is only supported on Linux")
}

func (unsupportedServer) Close() error   { return nil }
func (unsupportedServer) Addr() net.Addr { return nil }
