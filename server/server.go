/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
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
	"time"

	"github.com/quic-go/quic-go"
	"go.osspkg.com/errors"
	"go.osspkg.com/ioutils/fs"
	"go.osspkg.com/network/address"
	"go.osspkg.com/network/internal"
	"go.osspkg.com/network/listen"
	"go.osspkg.com/syncing"
	"go.osspkg.com/xc"
)

type (
	TServer interface {
		HandleFunc(h Handler)
		ListenAndServe(ctx xc.Context) error
	}

	Config struct {
		Address    string               `yaml:"address"`
		Network    string               `yaml:"network"`
		Certs      []listen.Certificate `yaml:"certs,omitempty"`
		Timeout    time.Duration        `yaml:"timeout,omitempty"`
		KeepAlive  time.Duration        `yaml:"keep_alive,omitempty"`
		BufferSize int                  `yaml:"buffer_size,omitempty"`
	}

	_server struct {
		conf     Config
		listener io.Closer
		handler  Handler
		sync     syncing.Switch
		wg       syncing.Group
	}
)

func New(conf Config) TServer {
	return &_server{
		conf: conf,
		sync: syncing.NewSwitch(),
		wg:   syncing.NewGroup(),
	}
}

func (v *_server) HandleFunc(h Handler) {
	if v.sync.IsOn() {
		return
	}
	v.handler = h
}

func (v *_server) ListenAndServe(ctx xc.Context) error {
	defer func() {
		ctx.Close()
	}()

	if v.handler == nil {
		return fmt.Errorf("handler not found")
	}
	if !v.sync.On() {
		return internal.ErrServAlreadyRunning
	}

	if err := v.build(ctx.Context()); err != nil {
		return err
	}

	if l, ok := v.listener.(*quic.Listener); ok {
		return v.handlingQUIC(ctx.Context(), l)
	}
	if l, ok := v.listener.(net.Listener); ok {
		return v.handlingConn(ctx.Context(), l)
	}
	if l, ok := v.listener.(net.PacketConn); ok {
		return v.handlingPacketConn(ctx.Context(), l)
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
		v.conf.Address = address.CheckHostPort(v.conf.Address)
	case internal.NetUDP:
		v.conf.Address = address.CheckHostPort(v.conf.Address)
	case internal.NetQUIC:
		v.conf.Address = address.CheckHostPort(v.conf.Address)
	case internal.NetUNIX:
		if fs.FileExist(v.conf.Address) {
			if err := os.Remove(v.conf.Address); err != nil {
				return errors.Wrapf(err, "fail clean socket file")
			}
		}
	}

	v.conf.Timeout = internal.NotZeroDuration(v.conf.Timeout, 1*time.Second)
	v.conf.KeepAlive = internal.NotZeroDuration(v.conf.KeepAlive, 15*time.Second)
	v.conf.BufferSize = internal.NotZero[int](v.conf.BufferSize, 65535)

	l, err := listen.New(ctx, v.conf.Network, v.conf.Address, v.conf.Certs...)
	if err != nil {
		return err
	}
	v.listener = l

	return nil
}

func (v *_server) handlingPacketConn(ctx context.Context, l net.PacketConn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		v.wg.Wait()
	}()

	v.wg.Background(func() {
		<-ctx.Done()
		v.close()
	})

	buff := make([]byte, v.conf.BufferSize)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, addr, err := l.ReadFrom(buff)
		if err != nil {
			internal.WriteErrLog("PacketConn: read message", err, addr)
			return err
		}
		if n == 0 {
			internal.WriteErrLog("PacketConn: read message", fmt.Errorf("empty request"), addr)
			continue
		}

		cp := poolPRC.Get()
		cp.Setup(ctx, l, addr)

		if _, err = cp.Pickup(buff[:n]); err != nil {
			poolPRC.Put(cp)
			internal.WriteErrLog("PacketConn: read message", err, addr)
			continue
		}

		v.wg.Background(func() {
			defer func() {
				if e := recover(); e != nil {
					internal.WriteErrLog("PacketConn: panic", fmt.Errorf("%+v", e), addr)
				}
				defer poolPRC.Put(cp)
			}()

			v.handler.Handler(cp)

			if _, e := cp.Release(); e != nil {
				internal.WriteErrLog("PacketConn: write message", e, addr)
			}
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
			cancel()
			return err
		}

		addr := conn.RemoteAddr()

		if tc, ok := conn.(*tls.Conn); ok {
			if err = tc.HandshakeContext(ctx); err != nil {
				internal.WriteErrLog("Conn: handshake", err, addr)
				conn.Close() //nolint: errcheck
				continue
			}
		}

		v.wg.Background(func() {
			bgCtx, bgCancel := context.WithCancel(ctx)
			defer func() {
				if e := recover(); e != nil {
					internal.WriteErrLog("Conn: panic", fmt.Errorf("%+v", e), addr)
				}
				conn.Close() //nolint: errcheck
				bgCancel()
			}()

			cp := poolRWC.Get()
			defer poolRWC.Put(cp)

			cp.Setup(bgCtx, v.conf.BufferSize, conn, addr)

			for {
				if e := internal.Deadline(conn, v.conf.KeepAlive); e != nil {
					internal.WriteErrLog("Conn: update keepalive", e, addr)
					return
				}

				if e := cp.Pickup(); e != nil {
					internal.WriteErrLog("Conn: read message", e, addr)
					return
				}

				if e := internal.Deadline(conn, v.conf.Timeout); e != nil {
					internal.WriteErrLog("Conn: update timeout", e, addr)
					return
				}

				v.handler.Handler(cp)

				if e := cp.Release(); e != nil {
					internal.WriteErrLog("Conn: write message", e, addr)
					return
				}
			}
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
			cancel()
			return err
		}

		addr := conn.RemoteAddr()

		v.wg.Background(func() {
			bgCtx, bgCancel := context.WithCancel(ctx)
			defer func() {
				if e := recover(); e != nil {
					internal.WriteErrLog("QUIC: panic", fmt.Errorf("%+v", e), addr)
				}
				bgCancel()
			}()

			stream, e := conn.AcceptStream(bgCtx)
			if e != nil {
				internal.WriteErrLog("QUIC: read message", e, addr)
				return
			}
			defer stream.Close() //nolint: errcheck

			cp := poolRWC.Get()
			defer poolRWC.Put(cp)

			cp.Setup(bgCtx, v.conf.BufferSize, stream, addr)

			for {
				if e = internal.Deadline(stream, v.conf.KeepAlive); e != nil {
					internal.WriteErrLog("QUIC: update keepalive", e, addr)
					return
				}

				if e = cp.Pickup(); e != nil {
					internal.WriteErrLog("QUIC: read message", e, addr)
					return
				}

				if e = internal.Deadline(stream, v.conf.Timeout); e != nil {
					internal.WriteErrLog("QUIC: update timeout", e, addr)
					return
				}

				v.handler.Handler(cp)

				if e = cp.Release(); e != nil {
					internal.WriteErrLog("QUIC: write message", e, addr)
					return
				}
			}
		})
	}
}
