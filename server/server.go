/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"

	"go.osspkg.com/errors"
	"go.osspkg.com/ioutils/fs"
	"go.osspkg.com/syncing"

	"go.osspkg.com/network/errs"
	"go.osspkg.com/network/internal"
	"go.osspkg.com/network/listen"
	"go.osspkg.com/network/packet"
	"go.osspkg.com/network/xio"
	"go.osspkg.com/network/xnet"
)

type (
	Server interface {
		HandleFunc(func(ctx context.Context, w io.Writer, r io.Reader, addr net.Addr))
		ListenAndServe(ctx context.Context) error
		Close() error
	}

	object struct {
		conf        Config
		listener    io.Closer
		handlerFunc func(ctx context.Context, w io.Writer, r io.Reader, addr net.Addr)
		sync        syncing.Switch
		wg          syncing.Group
	}
)

func New(c Config) Server {
	return &object{
		conf: c,
		sync: syncing.NewSwitch(),
		wg:   syncing.NewGroup(context.Background()),
	}
}

func (v *object) HandleFunc(fn func(context.Context, io.Writer, io.Reader, net.Addr)) {
	if v.sync.IsOn() {
		return
	}
	v.handlerFunc = fn
}

func (v *object) ListenAndServe(ctx context.Context) error {
	if v.handlerFunc == nil {
		return errors.New("handler not found")
	}
	if !v.sync.On() {
		return errs.ErrAlreadyRunning
	}

	addr, err := v.conf.Resolve()
	if err != nil {
		return err
	}
	v.conf.Address = addr.String()
	if v.conf.Network == xnet.NetUNIX && fs.FileExist(v.conf.Address) {
		if err = os.Remove(v.conf.Address); err != nil {
			return errors.Wrapf(err, "fail clean socket file")
		}
	}

	var ssl *tls.Config
	if v.conf.Certificate != nil {
		ssl, err = v.conf.Certificate.Parse()
		if err != nil {
			return errors.Wrapf(err, "fail load certificate")
		}
	}

	v.listener, err = listen.New(ctx, v.conf.Network, v.conf.Address, ssl)
	if err != nil {
		return errors.Wrapf(err, "fail create listener")
	}

	if l, ok := v.listener.(net.Listener); ok {
		return v.handlingConn(ctx, l)
	}
	if l, ok := v.listener.(net.PacketConn); ok {
		return v.handlingPacketConn(ctx, l)
	}

	return errors.New("unknown listener")
}

func (v *object) Close() error {
	if !v.sync.Off() {
		return nil
	}

	v.wg.Cancel()

	if v.listener == nil {
		return nil
	}
	return v.listener.Close()
}

func (v *object) handlingPacketConn(ctx context.Context, l net.PacketConn) error {
	defer v.wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	v.wg.Background("deadline auto update", func(ctx context.Context) {
		xio.AutoUpdate(ctx, l, 0)
	})

	v.wg.Background("auto close listener", func(wc context.Context) {
		select {
		case <-wc.Done():
		case <-ctx.Done():
		}
		internal.Log("PacketConn: close listener", v.Close(), nil)
	})

	buff := make([]byte, packet.Size)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, addr, err := l.ReadFrom(buff)
		if err != nil {
			internal.Log("PacketConn: read message", err, addr)
			return errors.Wrapf(err, "fail read packet form: %s", addr.String())
		}

		req := internal.DataPool.Get()

		if _, err = req.Write(buff[:n]); err != nil {
			internal.Log("PacketConn: read message", err, addr)
			return errors.Wrapf(err, "fail write packet form: %s", addr.String())
		}

		v.wg.Background("call handler", func(wc context.Context) {
			defer func() {
				if e := recover(); e != nil {
					internal.Log("PacketConn: panic", fmt.Errorf("%+v", e), addr)
				}

				internal.DataPool.Put(req)
			}()

			v.handlerFunc(ctx, &packet.Writer{Addr: addr, Conn: l}, req, addr)
		})
	}
}

func (v *object) handlingConn(ctx context.Context, l net.Listener) error {
	defer v.wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	v.wg.Background("auto close listener", func(wc context.Context) {
		select {
		case <-wc.Done():
		case <-ctx.Done():
		}
		internal.Log("Conn: close listener", v.Close(), nil)
	})

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn, err := l.Accept()
		if err != nil {
			internal.Log("Conn: accept", err, nil)
			return errors.Wrapf(err, "fail accept connection")
		}

		addr := conn.RemoteAddr()

		if tc, ok := conn.(*tls.Conn); ok {
			if err = tc.HandshakeContext(ctx); err != nil {
				internal.Log("Conn: handshake", err, addr)
				internal.Log("Conn: close", conn.Close(), addr)
				continue
			}
		}

		v.wg.Background("deadline auto update", func(ctx context.Context) {
			xio.AutoUpdate(ctx, conn, 0)
		})

		v.wg.Background("call handler", func(wc context.Context) {
			defer func() {
				if e := recover(); e != nil {
					internal.Log("Conn: panic", fmt.Errorf("%+v", e), addr)
				}

				internal.Log("Conn: close", conn.Close(), addr)
			}()

			v.handlerFunc(ctx, conn, conn, addr)
		})
	}
}
