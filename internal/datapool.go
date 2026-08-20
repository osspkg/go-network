/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import (
	"go.osspkg.com/bb"
	"go.osspkg.com/ioutils/pool"
)

var DataPool = pool.New[*bb.Buffer](func() *bb.Buffer {
	return bb.New(512)
})
