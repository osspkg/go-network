/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package errs

import (
	"io"
	"net/http"
	"strings"

	"go.osspkg.com/errors"
)

var (
	ErrAlreadyRunning     = errors.New("already running")
	ErrAlreadyStopped     = errors.New("already stopped")
	ErrInvalidCertificate = errors.New("invalid certificate")
)

func IsNormalClosed(err error) bool {
	if err == nil {
		return true
	}
	return IsClosed(err)
}

func IsClosed(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, io.EOF) || errors.Is(err, http.ErrServerClosed) {
		return true
	}

	strErr := strings.ToLower(err.Error())

	if strings.Contains(strErr, "i/o timeout") ||
		strings.Contains(strErr, "use of closed network connection") ||
		strings.Contains(strErr, "deadline exceeded") ||
		strings.Contains(strErr, "server closed") {
		return true
	}

	return false
}
