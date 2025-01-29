/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import "go.osspkg.com/logx"

func writeLog(err error, message, network, address string) {
	if err == nil {
		return
	}
	logx.Error(message, "err", err, "network", network, "address", address)
}
