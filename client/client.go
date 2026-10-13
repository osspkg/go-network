/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"context"
	"crypto/tls"
	"io"
	"net"

	"go.osspkg.com/errors"

	"go.osspkg.com/network/xio"
	"go.osspkg.com/network/xnet"
)

type (
	Client interface {
		Call(ctx context.Context, handler func(ctx context.Context, w io.Writer, r io.Reader) error) error
	}

	object struct {
		conf Config
		tls  *tls.Config
	}
)

func New(c Config) (Client, error) {
	addr, err := c.Resolve()
	if err != nil {
		return nil, errors.Wrapf(err, "fail resolve address")
	}

	c.Address = addr.String()

	cli := &object{
		conf: c,
	}

	if c.Certificate != nil {
		cli.tls, err = c.Certificate.Parse()
		if err != nil {
			return nil, errors.Wrapf(err, "fail parse certificate")
		}
	}

	return cli, nil
}

func (v *object) conn(ctx context.Context) (connect, error) {
	if v.conf.Network == xnet.NetTCP && v.tls != nil {
		dial := &tls.Dialer{
			NetDialer: new(net.Dialer),
			Config:    v.tls,
		}

		conn, err := dial.DialContext(ctx, v.conf.Network, v.conf.Address)
		if err != nil {
			return nil, errors.Wrapf(err, "fail dial connection: %s", v.conf.Address)
		}

		return conn, nil
	}

	var dial net.Dialer
	conn, err := dial.DialContext(ctx, v.conf.Network, v.conf.Address)
	if err != nil {
		return nil, errors.Wrapf(err, "fail dial connection: %s", v.conf.Address)
	}

	return conn, nil
}

func (v *object) Call(ctx context.Context, handler func(ctx context.Context, w io.Writer, r io.Reader) error) error {
	conn, err := v.conn(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go xio.AutoUpdate(ctx, conn, 0)

	err = handler(ctx, conn, conn)

	return errors.Wrap(err, conn.Close())
}
