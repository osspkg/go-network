/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

// Run a small echo check against a TCP, UDP, or UNIX endpoint:
//
//	go run ./examples/client -network tcp -address 127.0.0.1:8080
//	go run ./examples/client -network unix -address /tmp/network.sock -size 1048576
//	go run ./examples/client -network udp -address 127.0.0.1:8081 -count 10
//
// The endpoint must echo each request. UDP additionally requires a peer that
// supports the client's GONETUD1 fragmentation protocol.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.osspkg.com/errors"

	"go.osspkg.com/network/client"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "connection check failed:", err)
		os.Exit(1)
	}
}

func run() error {
	network := flag.String("network", envOr("NETWORK", "tcp"), "transport: tcp, udp, or unix")
	address := flag.String("address", envOr("ADDRESS", "127.0.0.1:8080"), "server address")
	message := flag.String("message", "go-network connection check", "text pattern used to build the request")
	size := flag.Int("size", 64<<10, "request size in bytes")
	count := flag.Int("count", 1, "number of echo requests")
	parallel := flag.Int("parallel", 1, "maximum concurrent requests and pooled connections")
	timeout := flag.Duration("timeout", 5*time.Second, "timeout for each request")
	flag.Parse()

	if *size <= 0 {
		return errors.New("-size must be positive")
	}
	if *message == "" {
		return errors.New("-message must not be empty")
	}
	if *count <= 0 {
		return errors.New("-count must be positive")
	}
	if *parallel <= 0 {
		return errors.New("-parallel must be positive")
	}
	if *timeout <= 0 {
		return errors.New("-timeout must be positive")
	}
	if *network == "udp" && *size > (16<<20)-4 {
		return errors.New("UDP request payload must not exceed 16 MiB minus 4 bytes")
	}

	payload := make([]byte, *size)
	pattern := []byte(*message)
	for i := range payload {
		payload[i] = pattern[i%len(pattern)]
	}

	cli, err := client.New(
		client.Config{Address: *address, Network: *network},
		client.WithPoolSize(*parallel),
	)
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	started := time.Now()
	var (
		completed atomic.Int64
		firstErr  error
		errOnce   sync.Once
		workers   sync.WaitGroup
	)

	jobs := make(chan int)
	for range min(*parallel, *count) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for request := range jobs {
				if err := checkEcho(cli, payload, *timeout); err != nil {
					errOnce.Do(func() { firstErr = errors.Wrapf(err, "request %d", request+1) })
					continue
				}
				completed.Add(1)
			}
		}()
	}

	for request := range *count {
		jobs <- request
	}
	close(jobs)
	workers.Wait()

	elapsed := time.Since(started)
	digest := sha256.Sum256(payload)
	fmt.Printf("Transport: %s %s\n", strings.ToUpper(*network), *address)
	fmt.Printf("Result:    %d/%d requests echoed %d bytes\n", completed.Load(), *count, len(payload))
	fmt.Printf("Elapsed:   %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("SHA-256:   %x\n", digest)

	return firstErr
}

func checkEcho(cli client.Client, payload []byte, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return cli.Call(ctx, func(rw io.ReadWriter) error {
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
		if _, err := io.Copy(rw, bytes.NewReader(header[:])); err != nil {
			return errors.Wrapf(err, "write request length")
		}
		if _, err := io.Copy(rw, bytes.NewReader(payload)); err != nil {
			return errors.Wrapf(err, "write request")
		}

		if _, err := io.ReadFull(rw, header[:]); err != nil {
			return errors.Wrapf(err, "read echo response length")
		}
		if size := binary.BigEndian.Uint32(header[:]); size != uint32(len(payload)) {
			return errors.Wrap(errors.New(fmt.Sprintf("echo response size differs: got %d, want %d", size, len(payload))))
		}
		response := make([]byte, len(payload))
		if _, err := io.ReadFull(rw, response); err != nil {
			return errors.Wrapf(err, "read echo response")
		}
		if !bytes.Equal(response, payload) {
			return errors.New("echo response differs from request")
		}
		return nil
	})
}

func envOr(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
