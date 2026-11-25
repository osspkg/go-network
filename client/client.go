/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"context"
	"crypto/tls"
	"io"
	"net"

	"go.osspkg.com/errors"

	"go.osspkg.com/network/proto/udpframe"
	"go.osspkg.com/network/xnet"
)

type object struct {
	conf Config
	tls  *tls.Config
	pool *connPool
}

// New creates a client with a bounded connection pool. Use WithPoolSize and
// WithIdleTimeout to configure its limits.
func New(c Config, options ...Option) (Client, error) {
	if err := xnet.IsPassableNetwork(c.Network); err != nil {
		return nil, err
	}
	if c.Address == "" {
		return nil, errors.New("client address is empty")
	}
	switch c.Network {
	case xnet.NetTCP, xnet.NetUDP:
		if _, _, err := net.SplitHostPort(c.Address); err != nil {
			return nil, errors.Wrapf(err, "invalid %s address %q", c.Network, c.Address)
		}
	case xnet.NetUNIX:
		if _, err := net.ResolveUnixAddr(xnet.NetUNIX, c.Address); err != nil {
			return nil, errors.Wrapf(err, "invalid unix address %q", c.Address)
		}
	}

	opts := defaultOptions()
	for _, option := range options {
		if option == nil {
			return nil, errors.New("client option is nil")
		}
		if err := option(&opts); err != nil {
			return nil, errors.Wrapf(err, "invalid client option")
		}
	}

	cli := &object{conf: c}
	if c.Certificate != nil {
		if c.Network != xnet.NetTCP {
			return nil, errors.New("TLS certificate configuration is only supported with TCP")
		}
		config, err := c.Certificate.Parse()
		if err != nil {
			return nil, errors.Wrapf(err, "fail parse certificate")
		}
		cli.tls = config
	}

	cli.pool = newConnPool(opts.poolSize, opts.idleTimeout, cli.dial)
	return cli, nil
}

func (v *object) dial(ctx context.Context) (net.Conn, error) {
	if v.conf.Network == xnet.NetTCP && v.tls != nil {
		dialer := &tls.Dialer{
			NetDialer: new(net.Dialer),
			Config:    v.tls,
		}
		conn, err := dialer.DialContext(ctx, v.conf.Network, v.conf.Address)
		if err != nil {
			return nil, errors.Wrapf(err, "fail dial connection %s", v.conf.Address)
		}
		return conn, nil
	}

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, v.conf.Network, v.conf.Address)
	if err != nil {
		return nil, errors.Wrapf(err, "fail dial connection %s", v.conf.Address)
	}
	if v.conf.Network == xnet.NetUDP {
		return udpframe.NewDatagramConn(conn), nil
	}
	return conn, nil
}

// Call borrows one connection from the pool for the duration of handler.
// Stream handlers must read the complete response before returning so a
// connection can be safely reused. The handler must not retain rw after
// returning. UDP uses the GONETUD1 framing format for all messages and requires
// the peer to echo each request ID in its response. A handler error discards
// the connection.
func (v *object) Call(ctx context.Context, handler func(io.ReadWriter) error) error {
	if ctx == nil {
		return errors.New("client context is nil")
	}
	if handler == nil {
		return errors.New("client handler is nil")
	}

	pooled, err := v.pool.acquire(ctx)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		v.pool.release(pooled, false)
		return err
	}

	if udpConn, ok := pooled.conn.(*udpframe.DatagramConn); ok {
		udpConn.BeginCall()
	}
	stopCancel := context.AfterFunc(ctx, func() {
		_ = pooled.conn.Close()
	})
	reusable := false
	defer func() {
		stopped := stopCancel()
		v.pool.release(pooled, reusable && stopped)
	}()
	if err = ctx.Err(); err != nil {
		return err
	}

	if err = handler(pooled.conn); err != nil {
		return err
	}
	if udpConn, ok := pooled.conn.(*udpframe.DatagramConn); ok && udpConn.RequestStarted() {
		if err = udpConn.FlushRequest(); err != nil {
			return err
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	reusable = true
	return nil
}

// Close closes idle and active connections and rejects future Call invocations.
func (v *object) Close() error {
	return v.pool.Close()
}
