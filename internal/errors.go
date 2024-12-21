/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import (
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/quic-go/quic-go"
	"go.osspkg.com/errors"
	"go.osspkg.com/logx"
)

var (
	ErrServAlreadyRunning = errors.New("server already running")
)

func NormalCloseError(err error) error {
	if IsNormalCloseError(err) {
		return nil
	}
	return err
}

func IsNormalCloseError(err error) bool {
	if err == nil ||
		errors.Is(err, io.EOF) ||
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

func WriteErrLog(message string, err error, addr net.Addr) {
	if err == nil || IsNormalCloseError(err) {
		return
	}
	logx.Warn(message, "err", err, "addr", addr)
}
