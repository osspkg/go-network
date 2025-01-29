/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package main

import (
	"fmt"
	"io"
	"os"

	"go.osspkg.com/logx"
	"go.osspkg.com/xc"

	"go.osspkg.com/network/listen"
	"go.osspkg.com/network/server"
)

func main() {
	logx.SetLevel(logx.LevelDebug)

	config := &server.Config{
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

	srv := server.New(*config)

	srv.HandleFunc(Handler)

	if err := srv.ListenAndServe(xc.New()); err != nil {
		panic(err)
	}
}

func Handler(ctx server.Ctx) {
	b, err := io.ReadAll(ctx)
	fmt.Println("[------", ctx.Addr(), "------]", err, string(b))
	ctx.Write(b)
}
