/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"time"

	"go.osspkg.com/errors"
)

const (
	defaultPoolSize    = 32
	defaultIdleTimeout = 2 * time.Minute
)

type options struct {
	poolSize    int
	idleTimeout time.Duration
}

// Option configures the connection pool created by New.
type Option func(*options) error

// WithPoolSize sets the maximum number of open connections, including idle
// ones. The default is 32.
func WithPoolSize(size int) Option {
	return func(opts *options) error {
		if size < 1 {
			return errors.New("pool size must be positive")
		}
		opts.poolSize = size
		return nil
	}
}

// WithIdleTimeout sets how long an idle connection stays in the pool before it
// is closed. The default is two minutes.
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
	return options{
		poolSize:    defaultPoolSize,
		idleTimeout: defaultIdleTimeout,
	}
}
