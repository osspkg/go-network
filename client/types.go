/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"context"
	"io"
)

// Client runs request handlers over pooled connections. Call handlers must
// consume a complete response before returning if the stream connection is to
// be reused.
type Client interface {
	Call(ctx context.Context, handler func(io.ReadWriter) error) error
	Close() error
}
