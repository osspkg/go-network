/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

// Run an echo server for the client example. TCP and UNIX use a four-byte
// big-endian length prefix followed by the payload. UDP messages use the same
// application format inside the client's GONETUD1 transport frames.
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go.osspkg.com/errors"

	"go.osspkg.com/network/server"
)

const maxEchoMessage = 16 << 20

func main() {
	config := server.Config{
		Address: envOr("ADDRESS", "127.0.0.1:8080"),
		Network: envOr("NETWORK", "tcp"),
	}

	srv, err := server.New(config)
	if err != nil {
		log.Fatal(err)
	}
	srv.HandleFunc(echo)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("listening on %s %s", config.Network, config.Address)
	if err := srv.ListenAndServe(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

func echo(_ context.Context, rw io.ReadWriter, addr net.Addr) error {
	for {
		var header [4]byte
		if _, err := io.ReadFull(rw, header[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return errors.Wrapf(err, "read request length from %s", addr)
		}
		size := binary.BigEndian.Uint32(header[:])
		if size > maxEchoMessage {
			return errors.Wrap(errors.New(fmt.Sprintf("request from %s is too large: %d bytes", addr, size)))
		}

		payload := make([]byte, int(size))
		if _, err := io.ReadFull(rw, payload); err != nil {
			return errors.Wrapf(err, "read request from %s", addr)
		}
		if _, err := io.Copy(rw, bytes.NewReader(header[:])); err != nil {
			return errors.Wrapf(err, "write response length to %s", addr)
		}
		if _, err := io.Copy(rw, bytes.NewReader(payload)); err != nil {
			return errors.Wrapf(err, "write response to %s", addr)
		}
	}
}

func envOr(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
