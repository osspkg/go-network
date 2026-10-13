/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package xio

import (
	"context"
	"time"
)

const (
	minimalInterval = time.Millisecond * 200
)

type Deadline interface {
	SetDeadline(t time.Time) error
}

func AutoUpdate(ctx context.Context, conn Deadline, interval time.Duration) {
	interval = max(interval, minimalInterval)

	tik := time.NewTicker(interval / 2)
	defer tik.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case v := <-tik.C:
			if err := conn.SetDeadline(v.Add(interval)); err != nil {
				return
			}
		}
	}
}
