/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

// Package epoll provides a Linux TCP server driven by an epoll event loop.
package epoll

import (
	"context"
	"net"
	"runtime"
	"time"

	"go.osspkg.com/errors"
)

// Config defines the TCP endpoint and callbacks. OnData receives arbitrary
// chunks of the TCP byte stream; it does not receive framed messages.
type Config struct {
	Address   string
	OnConnect func(*Conn) error
	OnData    func(*Conn, []byte) error
	OnClose   func(*Conn, error)
}

// Server runs a single-threaded event loop and invokes callbacks synchronously
// from that loop. Callbacks must be short and non-blocking.
type Server interface {
	ListenAndServe(context.Context) error
	Close() error
	Addr() net.Addr
}

// Option configures reactor resource limits.
type Option func(*options) error

type options struct {
	maxConnections int
	idleTimeout    time.Duration
}

// WithMaxConnections limits open client connections. The default is min of
// GOMAXPROCS and 256.
func WithMaxConnections(limit int) Option {
	return func(opts *options) error {
		if limit < 1 || limit > 4096 {
			return errors.New("max connections must be between 1 and 4096")
		}
		opts.maxConnections = limit
		return nil
	}
}

// WithIdleTimeout closes connections with no successful reads or writes for
// the specified duration. The default is two minutes.
func WithIdleTimeout(timeout time.Duration) Option {
	return func(opts *options) error {
		if timeout <= 0 {
			return errors.New("idle timeout must be positive")
		}
		opts.idleTimeout = timeout
		return nil
	}
}

func defaultOptions() options {
	limit := runtime.GOMAXPROCS(0)
	if limit > 256 {
		limit = 256
	}
	if limit < 1 {
		limit = 1
	}
	return options{maxConnections: limit, idleTimeout: 2 * time.Minute}
}

func validate(c Config, opts []Option) (optionsValue options, err error) {
	if c.Address == "" {
		return optionsValue, errors.New("epoll server address is empty")
	}
	if _, _, err := net.SplitHostPort(c.Address); err != nil {
		return optionsValue, errors.Wrapf(err, "invalid TCP address %q", c.Address)
	}
	optionsValue = defaultOptions()
	for _, option := range opts {
		if option == nil {
			return options{}, errors.New("epoll server option is nil")
		}
		if err := option(&optionsValue); err != nil {
			return options{}, errors.Wrapf(err, "invalid epoll server option")
		}
	}
	return optionsValue, nil
}
