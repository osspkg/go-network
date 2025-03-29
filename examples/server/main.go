/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"go.osspkg.com/ioutils/data"
	"go.osspkg.com/logx"

	"go.osspkg.com/network/listen"
	"go.osspkg.com/network/server"
)

func main() {
	logx.SetLevel(logx.LevelDebug)

	config := server.Config{
		Address: os.Getenv("ADDRESS"),
		Network: os.Getenv("NETWORK"),
	}

	if config.Network == "quic" {
		config.SSL = &server.SSL{
			Certs: []listen.Certificate{
				{AutoGenerate: true, Addresses: []string{"127.0.0.1"}},
			},
		}
	}

	srv := server.New(config)

	srv.HandleFunc(func(ctx context.Context, w io.Writer, r io.Reader, addr net.Addr) {
		buff := data.NewBuffer(1024)
		_, err := buff.ReadFrom(r)
		fmt.Println("[------", addr.String(), "------]", err, buff.String())
		buff.Seek(0, 0)
		buff.WriteTo(w)
	})

	if err := srv.ListenAndServe(context.TODO()); err != nil {
		panic(err)
	}
}
