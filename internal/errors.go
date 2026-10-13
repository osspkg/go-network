/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import (
	"net"

	"go.osspkg.com/logx"

	"go.osspkg.com/network/errs"
)

func Log(message string, err error, addr net.Addr) {
	if errs.IsNormalClosed(err) {
		return
	}
	if addr == nil {
		logx.Warn(message, "err", err)
	} else {
		logx.Warn(message, "err", err, "addr", addr)
	}
}
