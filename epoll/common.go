/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package epoll

import (
	"bytes"
	"io"
	"strings"

	"go.osspkg.com/errors"
	"go.osspkg.com/ioutils/pool"
	"golang.org/x/sys/unix"
)

const (
	epollEvents = unix.POLLIN | unix.POLLRDHUP | unix.POLLERR | unix.POLLHUP | unix.POLLNVAL
)

var (
	connPool = pool.NewSlicePool[int32](0, 30)
	buffPool = pool.New[*bytes.Buffer](func() *bytes.Buffer {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	})
)

func isClosedError(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "use of closed network connection") ||
		errors.Is(err, io.EOF) {
		return true
	}
	return false
}
