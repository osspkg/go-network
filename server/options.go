/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"runtime"
	"time"

	"go.osspkg.com/errors"
)

const maxConnectionsLimit = 4096

type options struct {
	maxConnections int
	idleTimeout    time.Duration
}

// Option configures server resource limits.
type Option func(*options) error

// WithMaxConnections limits simultaneous TCP/UNIX handlers or UDP handlers.
// The default is min(GOMAXPROCS, 256).
func WithMaxConnections(limit int) Option {
	return func(opts *options) error {
		if limit < 1 || limit > maxConnectionsLimit {
			return errors.New("max connections must be between 1 and 4096")
		}
		opts.maxConnections = limit
		return nil
	}
}

// WithIdleTimeout closes a TCP or UNIX connection after this long without
// successful reads or writes. The default is two minutes.
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
