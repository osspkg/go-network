/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"go.osspkg.com/ioutils/data"
	"go.osspkg.com/syncing"

	"go.osspkg.com/network/client"
)

func main() {
	config := client.Config{
		Address:  os.Getenv("ADDRESS"),
		Network:  os.Getenv("NETWORK"),
		MaxConns: 10,
	}

	if config.Network == "quic" {
		config.Certificate = &client.Certificate{InsecureSkipVerify: true}
	}

	cli, err := client.New(config)
	if err != nil {
		panic(err)
	}

	var (
		good int64
		fail int64
	)

	for i := 0; i < 3; i++ {
		fmt.Println("------------ STEP", i, "---------------")
		wg := syncing.NewGroup()
		for i := 0; i < 10000; i++ {
			i := i
			wg.Background(func() {
				buff := data.NewBuffer(1024)
				buff.WriteString(fmt.Sprintf("<- %d ->", i))
				err := cli.Call(context.TODO(), func(ctx context.Context, w io.Writer, r io.Reader) error {
					if _, err := buff.WriteTo(w); err != nil {
						return err
					}
					buff.Reset()
					if _, err := buff.ReadFrom(r); err != nil {
						return err
					}
					return nil
				})
				if err != nil {
					fmt.Println(i, "E", err)
					atomic.AddInt64(&fail, 1)
				} else {
					fmt.Println(i, "B", buff.String())
					atomic.AddInt64(&good, 1)
				}

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
