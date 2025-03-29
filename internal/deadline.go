/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import (
	"io"
	"time"
)

type Conn interface {
	io.ReadWriteCloser
	Deadline
}

type Deadline interface {
	SetDeadline(t time.Time) error
}

func DeadlineUpdate(conn Deadline) func() {
	tik := time.NewTicker(time.Second * 5)
	closeC := make(chan struct{})

	go func() {
		for {
			select {
			case <-closeC:
				return
			case v := <-tik.C:
				if err := conn.SetDeadline(v.Add(time.Second * 10)); err != nil {
					return
				}
			}
		}
	}()

	return tik.Stop
}
