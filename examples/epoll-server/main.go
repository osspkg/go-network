/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package main

import (
	"context"
	"io"
	"os"

	"go.osspkg.com/logx"
	"go.osspkg.com/network/epoll"
	"go.osspkg.com/xc"
)

func main() {
	logger := logx.New()
	logger.SetLevel(logx.LevelDebug)
	logger.SetFormatter(logx.NewFormatString())
	logger.SetOutput(os.Stdout)

	serv := &epoll.ServerTCP{
		Handler: func(_ context.Context, w io.Writer, r io.Reader) error {
			b, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			_, err = w.Write(append([]byte(">> "), b...))
			return err
		},
		Logger: logger,
		Config: epoll.ConfigTCP{
			Addr:           "127.0.0.1:11111",
			CountEvents:    100,
			WaitIntervalMS: 300,
		},
	}

	if err := serv.ListenAndServe(xc.New()); err != nil {
		panic(err)
	}
}