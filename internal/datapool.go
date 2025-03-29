/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import (
	"go.osspkg.com/ioutils/data"
	"go.osspkg.com/ioutils/pool"
)

var DataPool = pool.New[*data.Buffer](func() *data.Buffer {
	return data.NewBuffer(512)
})
