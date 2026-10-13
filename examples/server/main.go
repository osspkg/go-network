/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"go.osspkg.com/bb"

	"go.osspkg.com/network/server"
)

func main() {
	config := server.Config{
		Address: os.Getenv("ADDRESS"),
		Network: os.Getenv("NETWORK"),
	}

	srv := server.New(config)

	srv.HandleFunc(func(ctx context.Context, w io.Writer, r io.Reader, addr net.Addr) {
		buff := bb.New(1024)
		_, err := buff.ReadFrom(r)
		fmt.Println("[------", addr.String(), "------]", err, buff.String())
		buff.Seek(0, 0) //nolint:errcheck
		buff.WriteTo(w) //nolint:errcheck
	})

	if err := srv.ListenAndServe(context.TODO()); err != nil {
		panic(err)
	}
}
