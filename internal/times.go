/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import "time"

type Comparable interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

func NotZeroDuration(args ...time.Duration) time.Duration {
	for _, arg := range args {
		if arg > 0 {
			return arg
		}
	}
	return 0
}

func NotZero[T Comparable](args ...T) T {
	for _, arg := range args {
		if arg > 0 {
			return arg
		}
	}
	return 0
}
