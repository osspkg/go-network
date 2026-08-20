/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import (
	"io"
	"time"
)

const (
	minimalInterval = time.Millisecond * 200
)

type Conn interface {
	io.ReadWriteCloser
	Deadline
}

type Deadline interface {
	SetDeadline(t time.Time) error
}

func AutoUpdateDeadline(conn Deadline, interval time.Duration) func() {
	if interval <= minimalInterval {
		interval = minimalInterval
	}

	tik := time.NewTicker(interval / 2)
	closeC := make(chan struct{})

	go func() {
		for {
			select {
			case <-closeC:
				return
			case v := <-tik.C:
				if err := conn.SetDeadline(v.Add(interval)); err != nil {
					return
				}
			}
		}
	}()

	return tik.Stop
}
