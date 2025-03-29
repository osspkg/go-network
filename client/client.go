/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"

	"github.com/quic-go/quic-go"
	"go.osspkg.com/algorithms/control"
	"go.osspkg.com/errors"

	"go.osspkg.com/network/internal"
)

type (
	Client interface {
		Call(ctx context.Context, handler func(ctx context.Context, w io.Writer, r io.Reader) error) error
	}

	_client struct {
		conf Config
		tls  *tls.Config
		sem  control.Semaphore
	}
)

func New(c Config) (Client, error) {
	addr, err := c.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve address: %w", err)
	}

	c.Address = addr.String()

	tlsc, err := c.Certificate.Config(c.Address, c.Network)
	if err != nil {
		return nil, fmt.Errorf("get tls config: %w", err)
	}

	if c.MaxConns <= 0 {
		c.MaxConns = 1
	}

	cli := &_client{
		conf: c,
		sem:  control.NewSemaphore(c.MaxConns),
		tls:  tlsc,
	}

	return cli, nil
}

func (v *_client) conn(ctx context.Context) (internal.Conn, error) {
	switch v.conf.Network {
	case internal.NetQUIC:
		conn, err := quic.DialAddr(ctx, v.conf.Address, v.tls, &quic.Config{EnableDatagrams: false})
		if err != nil {
			return nil, fmt.Errorf("dial quic: %w", err)
		}

		stream, err := conn.OpenStreamSync(ctx)
		if err != nil {
			return nil, fmt.Errorf("open stream quic: %w", errors.Wrap(err, conn.CloseWithError(0, "")))
		}
		return &rwc{D: stream, R: stream, W: stream, C: func() error {
			return errors.Wrap(stream.Close(), conn.CloseWithError(0, ""))
		}}, nil

	case internal.NetTCP:
		if v.tls != nil {
			dial := &tls.Dialer{
				NetDialer: new(net.Dialer),
				Config:    v.tls,
			}
			conn, err := dial.DialContext(ctx, v.conf.Network, v.conf.Address)
			if err != nil {
				return nil, fmt.Errorf("dial tcp tls: %w", err)
			}
			return conn, nil
		}
		fallthrough

	default:
		var dial net.Dialer
		conn, err := dial.DialContext(ctx, v.conf.Network, v.conf.Address)
		if err != nil {
			return nil, fmt.Errorf("dial %s: %w", v.conf.Network, err)
		}
		return conn, nil
	}
}

func (v *_client) Call(ctx context.Context, handler func(ctx context.Context, w io.Writer, r io.Reader) error) (e error) {
	v.sem.Acquire()
	defer func() { v.sem.Release() }()

	conn, err := v.conn(ctx)
	if err != nil {
		return err
	}

	stop := internal.DeadlineUpdate(conn)

	defer func() {
		stop()
		e = errors.Wrap(e, conn.Close())
	}()

	e = handler(ctx, conn, conn)

	return
}
