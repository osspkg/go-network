/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"context"
	"io"
	"net"
)

// Handler processes one UDP message or a stream connection. For TCP and UNIX,
// the handler owns the stream protocol and may process multiple requests before
// returning. For UDP, rw reads one complete request and collects one response.
// UDP uses the GONETUD1 framing format shared with client and supports messages
// up to 16 MiB. It fragments messages into 1200-byte datagrams but does not
// retransmit lost fragments; the caller should use a request deadline.
type Handler func(ctx context.Context, rw io.ReadWriter, addr net.Addr) error

// Server listens for TCP, UNIX, or framed UDP requests.
type Server interface {
	HandleFunc(Handler)
	ListenAndServe(ctx context.Context) error
	Close() error
}
