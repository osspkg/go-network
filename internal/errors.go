/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import (
	"strings"

	"go.osspkg.com/errors"
)

var (
	ErrServAlreadyRunning = errors.New("server already running")
)

func NormalCloseError(err error) error {
	if err == nil ||
		strings.Contains(err.Error(), "use of closed network connection") {
		return nil
	}
	return err
}
