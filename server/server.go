/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
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

	"github.com/quic-go/quic-go"
	"go.osspkg.com/errors"
	"go.osspkg.com/ioutils/fs"
	"go.osspkg.com/syncing"

	"go.osspkg.com/network/address"
	"go.osspkg.com/network/internal"
	"go.osspkg.com/network/listen"
)

type (
	Server interface {
		HandleFunc(func(ctx context.Context, w io.Writer, r io.Reader, addr net.Addr))
		ListenAndServe(ctx context.Context) error
	}

	_server struct {
		conf        Config
		listener    io.Closer
		handlerFunc func(ctx context.Context, w io.Writer, r io.Reader, addr net.Addr)
		sync        syncing.Switch
		wg          syncing.Group
	}
)

func New(conf Config) Server {
	return &_server{
		conf: conf,
		sync: syncing.NewSwitch(),
		wg:   syncing.NewGroup(),
	}
}

func (v *_server) HandleFunc(fn func(context.Context, io.Writer, io.Reader, net.Addr)) {
	if v.sync.IsOn() {
		return
	}
	v.handlerFunc = fn
}

func (v *_server) ListenAndServe(ctx context.Context) error {
	if v.handlerFunc == nil {
		return fmt.Errorf("handler not found")
	}
	if !v.sync.On() {
		return internal.ErrServAlreadyRunning
	}

	if err := v.build(ctx); err != nil {
		return err
	}

	if l, ok := v.listener.(*quic.Listener); ok {
		return v.handlingQUIC(ctx, l)
	}
	if l, ok := v.listener.(net.Listener); ok {
		return v.handlingConn(ctx, l)
	}
	if l, ok := v.listener.(net.PacketConn); ok {
		return v.handlingPacketConn(ctx, l)
	}

	return fmt.Errorf("unknown listener")
}

func (v *_server) close() {
	if !v.sync.Off() {
		return
	}
	v.listener.Close() // nolint: errcheck
}

func (v *_server) build(ctx context.Context) error {
	switch v.conf.Network {
	case internal.NetTCP:
		v.conf.Address = address.ResolveIPPort(v.conf.Address)
	case internal.NetUDP:
		v.conf.Address = address.ResolveIPPort(v.conf.Address)
	case internal.NetQUIC:
		v.conf.Address = address.ResolveIPPort(v.conf.Address)
	case internal.NetUNIX:
		if fs.FileExist(v.conf.Address) {
			if err := os.Remove(v.conf.Address); err != nil {
				return errors.Wrapf(err, "fail clean socket file")
			}
		}
	}

	ssl := &listen.SSL{}
	if v.conf.SSL != nil {
		ssl.Certs = append(ssl.Certs, v.conf.SSL.Certs...)
		ssl.NextProtos = append(ssl.NextProtos, v.conf.SSL.NextProtos...)
	}

	l, err := listen.New(ctx, v.conf.Network, v.conf.Address, ssl)
	if err != nil {
		return err
	}
	v.listener = l

	return nil
}

func (v *_server) handlingPacketConn(ctx context.Context, l net.PacketConn) error {
	ctx, cancel := context.WithCancel(ctx)

	stop := internal.DeadlineUpdate(l)

	defer func() {
		stop()
		cancel()
		v.wg.Wait()
	}()

	v.wg.Background(func() {
		<-ctx.Done()
		v.close()
	})

	buff := make([]byte, internal.UDPPacketSize)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, addr, err := l.ReadFrom(buff)
		if err != nil {
			internal.Log("PacketConn: read message", err, addr)
			return err
		}

		req := internal.DataPool.Get()

		if _, err = req.Write(buff[:n]); err != nil {
			internal.Log("PacketConn: read message", err, addr)
			return err
		}

		v.wg.Background(func() {
			defer func() {
				if e := recover(); e != nil {
					internal.Log("PacketConn: panic", fmt.Errorf("%+v", e), addr)
				}

				internal.DataPool.Put(req)
			}()

			v.handlerFunc(ctx, &internal.PacketWrite{Addr: addr, Conn: l}, req, addr)
		})
	}
}

func (v *_server) handlingConn(ctx context.Context, l net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)

	v.wg.Background(func() {
		<-ctx.Done()
		v.close()
	})

	defer func() {
		cancel()
		v.wg.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn, err := l.Accept()
		if err != nil {
			internal.Log("Conn: accept", err, nil)
			return err
		}

		addr := conn.RemoteAddr()

		if tc, ok := conn.(*tls.Conn); ok {
			if err = tc.HandshakeContext(ctx); err != nil {
				internal.Log("Conn: handshake", err, addr)
				internal.Log("Conn: close", conn.Close(), addr)
				continue
			}
		}

		v.wg.Background(func() {
			stop := internal.DeadlineUpdate(conn)

			defer func() {
				if e := recover(); e != nil {
					internal.Log("Conn: panic", fmt.Errorf("%+v", e), addr)
				}

				stop()

				internal.Log("Conn: close", conn.Close(), addr)
			}()

			v.handlerFunc(ctx, conn, conn, addr)
		})
	}
}

func (v *_server) handlingQUIC(ctx context.Context, l *quic.Listener) error {
	ctx, cancel := context.WithCancel(ctx)

	v.wg.Background(func() {
		<-ctx.Done()
		v.close()
	})

	defer func() {
		cancel()
		v.wg.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn, err := l.Accept(ctx)
		if err != nil {
			internal.Log("QUIC: accept", err, nil)
			return err
		}

		addr := conn.RemoteAddr()

		v.wg.Background(func() {
			defer func() {
				if e := recover(); e != nil {
					internal.Log("QUIC: panic", fmt.Errorf("%+v", e), addr)
				}

				//internal.Log("QUIC: close conn", conn.CloseWithError(0, ""), addr)
			}()

			stream, e := conn.AcceptStream(ctx)
			if e != nil {
				internal.Log("QUIC: read message", e, addr)
				return
			}

			stop := internal.DeadlineUpdate(stream)

			defer func() {
				stop()
				internal.Log("QUIC: close stream", stream.Close(), addr)
			}()

			v.handlerFunc(ctx, stream, stream, addr)
		})
	}
}
