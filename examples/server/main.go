/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package main

import (
	"fmt"
	"io"
	"os"

	"go.osspkg.com/logx"
	"go.osspkg.com/network/listen"
	"go.osspkg.com/network/server"
	"go.osspkg.com/xc"
)

func main() {
	logx.SetLevel(logx.LevelDebug)

	config := &server.Config{
		Address: os.Getenv("ADDRESS"),
		Network: os.Getenv("NETWORK"),
	}

	if config.Network == "quic" {
		config.Certs = append(config.Certs, listen.Certificate{AutoGenerate: true, Addresses: []string{"127.0.0.1"}})
	}

	srv := server.New(*config)

	echo := &Echo{}
	srv.HandleFunc(echo)

	if err := srv.ListenAndServe(xc.New()); err != nil {
		panic(err)
	}
}

type Echo struct {
}

func (*Echo) Handler(ctx server.Ctx) {
	b, err := io.ReadAll(ctx)
	fmt.Println("[------", ctx.Addr(), "------]", err, string(b))
	ctx.Write(b)
}
