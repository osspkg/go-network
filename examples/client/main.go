/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"go.osspkg.com/network/client"
	"go.osspkg.com/syncing"
)

func main() {
	cli := &client.Client{
		Address:      os.Getenv("ADDRESS"),
		Network:      os.Getenv("NETWORK"),
		MaxIdleConns: 10,
	}

	if cli.Network == "quic" {
		cli.Certificate = &client.Certificate{InsecureSkipVerify: true}
	}

	var (
		good int64
		fail int64
	)

	for i := 0; i < 3; i++ {
		fmt.Println("------------ STEP", i, "---------------")
		wg := syncing.NewGroup()
		for i := 0; i < 100000; i++ {
			i := i
			wg.Background(func() {
				in := bytes.NewBufferString(fmt.Sprintf("<- %d ->", i))
				out := bytes.NewBuffer(nil)
				if err := cli.Do(in, out); err != nil {
					fmt.Println(i, "ERR", err)
					atomic.AddInt64(&fail, 1)
					return
				}
				b, err := io.ReadAll(out)
				fmt.Println(i, "E", err, "B", string(b))
				atomic.AddInt64(&good, 1)
			})
		}
		wg.Wait()

		time.Sleep(5 * time.Second)
	}

	fmt.Print(
		"\n-------------------------\n",
		"good\t", good, "\tfail\t", fail,
		"\n-------------------------\n",
	)
}
