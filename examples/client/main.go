/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"go.osspkg.com/network/client"
	"go.osspkg.com/syncing"
)

func main() {
	cli := &client.Client{
		Address: "127.0.0.1:8888",
		Timeout: 1 * time.Second,
		Network: "tcp",
	}

	ctx, _ := context.WithTimeout(context.TODO(), 15*time.Second)

	wg := syncing.NewGroup()
	for i := 0; i < 10000; i++ {
		i := i
		wg.Background(func() {
			in := bytes.NewBufferString("<-->")
			out := bytes.NewBuffer(nil)
			if err := cli.Do(ctx, in, out); err != nil {
				fmt.Println(i, "ERR", err)
				return
			}
			b, err := io.ReadAll(out)
			fmt.Println(i, "E", err, "B", string(b))
		})
	}
	wg.Wait()
}
