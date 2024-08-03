/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"go.osspkg.com/errors"
	"go.osspkg.com/ioutils"
	"go.osspkg.com/network/internal"
)

type (
	Client struct {
		Address string
		Timeout time.Duration
		Network string
	}
)

func (v *Client) dialConnect(ctx context.Context) (net.Conn, error) {
	var (
		addr fmt.Stringer
		err  error
	)
	if err = internal.IsPassableNetwork(v.Network); err != nil {
		return nil, err
	}
	switch v.Network {
	case "tcp":
		addr, err = net.ResolveTCPAddr("tcp", v.Address)
	case "udp":
		addr, err = net.ResolveUDPAddr("udp", v.Address)
	case "unix":
		addr, err = net.ResolveUnixAddr("udp", v.Address)
	default:
		return nil, fmt.Errorf("invalid network name, use: tcp, udp, unix")
	}
	if err != nil {
		return nil, fmt.Errorf("invalid address")
	}

	var dial net.Dialer
	conn, err := dial.DialContext(ctx, v.Network, addr.String())
	if err != nil {
		return nil, fmt.Errorf("create connect: %w", err)
	}
	return conn, nil
}

func (v *Client) Do(ctx context.Context, in io.Reader, out io.Writer) error {
	conn, err := v.dialConnect(ctx)
	if err != nil {
		return err
	}

	defer func() {
		conn.Close() // nolint: errcheck
	}()

	ttl := internal.NotZeroDuration(v.Timeout, 2*time.Second)

	t := time.Now().Add(ttl)
	err = errors.Wrap(conn.SetDeadline(t), conn.SetReadDeadline(t), conn.SetWriteDeadline(t))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, ttl)
	defer cancel()

	if _, err = ioutils.Copy(conn, in); err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	rdC := make(chan interface{}, 1)
	go func() {
		defer close(rdC)
		if _, e := ioutils.Copy(out, conn); e != nil {
			rdC <- fmt.Errorf("read message: %w", e)
		}
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("closed by timeout")
	case rcV := <-rdC:
		if e, ok := rcV.(error); ok {
			return e
		}
		return nil
	}
}
