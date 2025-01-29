/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"context"
	"io"
)

type (
	Ctx interface {
		io.Reader
		io.Writer
		Addr() string
		Context() context.Context
	}

	Handler interface {
		Handler(ctx Ctx)
	}
)
