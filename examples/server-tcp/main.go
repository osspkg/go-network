/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.osspkg.com/logx"
	"go.osspkg.com/network/server"
)

func main() {
	log := logx.New()
	log.SetLevel(logx.LevelDebug)

	srv := server.New(server.Config{
		Address: "127.0.0.1:8888",
		Certs:   nil,
		Timeout: 15 * time.Second,
		Network: "tcp",
	}, log)

	ctx, cancel := context.WithTimeout(context.TODO(), 120*time.Second)
	defer cancel()

	echo := &Echo{}
	srv.HandleFunc(echo)

	if err := srv.ListenAndServe(ctx); err != nil {
		panic(err)
	}
}

type Echo struct {
}

func (*Echo) Handler(w io.Writer, r io.Reader, addr string) {
	b, _ := io.ReadAll(r)
	fmt.Println("[IN]", string(b), "[addr]", addr)
	w.Write(b)
}
