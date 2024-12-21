/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package epoll

import (
	"context"
	"fmt"
	"io"
)

type (
	Option struct {
		Handler        func(ctx context.Context, w io.Writer, r io.Reader) error
		CountEvents    uint
		WaitIntervalMS uint
	}
)

func (c Option) Validate() error {
	if c.Handler == nil {
		return fmt.Errorf("epoll handler is empty")
	}
	if c.CountEvents == 0 {
		return fmt.Errorf("epoll count events is empty")
	}
	if c.WaitIntervalMS == 0 {
		return fmt.Errorf("epoll wait interval is empty")
	}
	return nil
}
