/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package errs

import (
	"io"
	"net/http"
	"strings"

	"github.com/quic-go/quic-go"
	"go.osspkg.com/errors"
)

func IsClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) ||
		errors.Is(err, quic.ErrServerClosed) ||
		errors.Is(err, http.ErrServerClosed) ||
		strings.Contains(err.Error(), "i/o timeout") ||
		strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "deadline exceeded") ||
		strings.Contains(err.Error(), "server closed") {
		return true
	}
	return false
}
