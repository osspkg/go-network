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
	"time"

	"go.osspkg.com/errors"
	"go.osspkg.com/logx"
	"go.osspkg.com/network/internal"
	"go.osspkg.com/network/listen"
	"go.osspkg.com/routine"
	"go.osspkg.com/syncing"
)

type (
	Handler interface {
		Handler(w io.Writer, r io.Reader, addr string)
	}

	TServer interface {
		HandleFunc(h Handler)
		ListenAndServe(ctx context.Context) error
	}

	Config struct {
		Address string        `yaml:"address"`
		Network string        `yaml:"network"`
		Certs   []listen.Cert `yaml:"certs,omitempty"`
		Timeout time.Duration `yaml:"timeout,omitempty"`
	}

	_server struct {
		conf     Config
		listener io.Closer
		handler  Handler
		log      logx.Logger
		sync     syncing.Switch
		wg       syncing.Group
	}
)

func New(conf Config, log logx.Logger) TServer {
	return &_server{
		conf: conf,
		log:  log,
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

func (v *_server) ListenAndServe(ctx context.Context) error {
	if v.handler == nil {
		return fmt.Errorf("handler not found")
	}
	if !v.sync.On() {
		return internal.ErrServAlreadyRunning
	}

	if err := v.build(ctx); err != nil {
		return err
	}

	if l, ok := v.listener.(net.Listener); ok {
		return v.handlingTCP(ctx, l)
	}
	if l, ok := v.listener.(net.PacketConn); ok {
		return v.handlingUDP(ctx, l)
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
	certs := make([]listen.Cert, 0, len(v.conf.Certs))
	for _, cert := range v.conf.Certs {
		certs = append(certs, listen.Cert{CertFile: cert.CertFile, KeyFile: cert.KeyFile})
	}
	l, err := listen.New(ctx, v.conf.Network, v.conf.Address, certs...)
	if err != nil {
		return err
	}
	v.listener = l
	return nil
}

func (v *_server) handlingUDP(ctx context.Context, l net.PacketConn) error {
	ctx, cancel := context.WithCancel(ctx)
	v.wg.Background(func() {
		<-ctx.Done()
		v.close()
	})
	defer func() {
		cancel()
		v.wg.Wait()
	}()

	routine.Interval(ctx, v.conf.Timeout/2, func(ctx context.Context) {
		t := time.Now().Add(v.conf.Timeout)
		err := errors.Wrap(l.SetDeadline(t), l.SetReadDeadline(t), l.SetWriteDeadline(t))
		if err != nil {
			v.log.WithFields(logx.Fields{
				"err": err.Error(),
			}).Warnf("update deadline")
			cancel()
			return
		}
	})

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		readBuff := internal.BuffPool.Get()
		n, addr, err := internal.CopyFrom(readBuff, l)
		if err != nil || n == 0 {
			internal.BuffPool.Put(readBuff)
			return err
		}

		v.wg.Background(func() {
			defer internal.BuffPool.Put(readBuff)

			select {
			case <-ctx.Done():
				return
			default:
			}

			writeBuff := internal.BuffPool.Get()
			v.handler.Handler(writeBuff, readBuff, addr.String())
			if _, err0 := internal.CopyTo(l, writeBuff, addr); err0 != nil {
				v.log.WithFields(logx.Fields{
					"err":  err0.Error(),
					"addr": addr.String(),
				}).Warnf("send message")
			}
		})
	}
}

func (v *_server) handlingTCP(ctx context.Context, l net.Listener) error {
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
			return internal.NormalCloseError(err)
		}

		t := time.Now().Add(v.conf.Timeout)
		err = errors.Wrap(conn.SetDeadline(t), conn.SetReadDeadline(t), conn.SetWriteDeadline(t))
		if err != nil {
			v.log.WithFields(logx.Fields{
				"err":  err.Error(),
				"addr": conn.RemoteAddr().String(),
			}).Warnf("update deadline")
			conn.Close() // nolint: errcheck
			continue
		}

		if tc, ok := conn.(*tls.Conn); ok {
			if err = tc.HandshakeContext(ctx); err != nil {
				v.log.WithFields(logx.Fields{
					"err":  err.Error(),
					"addr": conn.RemoteAddr().String(),
				}).Warnf("handshake")
				conn.Close() // nolint: errcheck
				continue
			}
		}

		v.wg.Background(func() {
			cp := connPool.Get()
			defer connPool.Put(cp)

			cp.Set(conn, v.conf.Timeout)

			select {
			case <-ctx.Done():
				conn.Close() // nolint: errcheck
				return
			default:
			}

			if err0 := cp.Wait(); err0 == nil {
				v.handler.Handler(cp, cp, cp.Addr())
				return
			} else {
				v.log.WithFields(logx.Fields{
					"err":  err.Error(),
					"addr": conn.RemoteAddr().String(),
				}).Warnf("read message")
			}
			conn.Close() // nolint: errcheck
		})
	}
}
