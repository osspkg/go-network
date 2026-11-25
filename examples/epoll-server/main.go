/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

// Run a Linux epoll TCP echo server:
//
//	go run ./examples/epoll-server -address 127.0.0.1:8082
//
// The server echoes each received TCP stream chunk as it arrives.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.osspkg.com/network/epoll"
)

func main() {
	address := flag.String("address", envOr("ADDRESS", "127.0.0.1:8082"), "TCP listen address")
	flag.Parse()

	srv, err := epoll.New(epoll.Config{
		Address: *address,
		OnConnect: func(conn *epoll.Conn) error {
			log.Printf("connected: %s", conn.RemoteAddr())
			return nil
		},
		OnData: func(conn *epoll.Conn, data []byte) error {
			return conn.Write(data)
		},
		OnClose: func(conn *epoll.Conn, cause error) {
			log.Printf("closed: %s (%v)", conn.RemoteAddr(), cause)
		},
	}, epoll.WithMaxConnections(256))
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("epoll echo server listening on %s", *address)
	if err := srv.ListenAndServe(ctx); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
