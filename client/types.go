/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"io"

	"go.osspkg.com/network/xio"
)

type connect interface {
	io.ReadWriteCloser
	xio.Deadline
}
