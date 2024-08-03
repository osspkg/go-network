/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import "time"

func NotZeroDuration(args ...time.Duration) time.Duration {
	for _, arg := range args {
		if arg != 0 {
			return arg
		}
	}
	return 0
}

func NotZeroUint64(args ...uint64) uint64 {
	for _, arg := range args {
		if arg != 0 {
			return arg
		}
	}
	return 0
}
