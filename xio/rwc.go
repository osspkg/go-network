/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package xio

import (
	"io"
	"time"
)

type ReadWriteClose struct {
	Deadline Deadline
	Reader   io.Reader
	Writer   io.Writer
	Closer   io.Closer
}

func (v *ReadWriteClose) Read(p []byte) (int, error) {
	return v.Reader.Read(p)
}

func (v *ReadWriteClose) Write(p []byte) (int, error) {
	return v.Writer.Write(p)
}

func (v *ReadWriteClose) Close() error {
	return v.Closer.Close()
}

func (v *ReadWriteClose) SetDeadline(t time.Time) error {
	return v.Deadline.SetDeadline(t)
}
