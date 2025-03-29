/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"io"
	"time"

	"go.osspkg.com/network/internal"
)

type rwc struct {
	D internal.Deadline
	R io.Reader
	W io.Writer
	C func() error
}

func (v *rwc) Read(p []byte) (int, error) {
	return v.R.Read(p)
}

func (v *rwc) Write(p []byte) (int, error) {
	return v.W.Write(p)
}

func (v *rwc) Close() error {
	return v.C()
}

func (v *rwc) SetDeadline(t time.Time) error {
	return v.D.SetDeadline(t)
}
