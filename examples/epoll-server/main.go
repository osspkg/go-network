/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package main

import (
	"context"
	"io"

	"go.osspkg.com/xc"

	"go.osspkg.com/network/epoll"
)

func main() {
	serv := &epoll.ServerTCP{
		Handler: func(_ context.Context, w io.Writer, r io.Reader) error {
			b, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			_, err = w.Write(append([]byte(">> "), b...))
			return err
		},
		Config: epoll.ConfigTCP{
			Addr:           "127.0.0.1:8888",
			CountEvents:    100,
			WaitIntervalMS: 300,
		},
	}

	if err := serv.ListenAndServe(xc.New()); err != nil {
		panic(err)
	}
}
