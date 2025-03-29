/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package epoll

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"go.osspkg.com/errors"
	"go.osspkg.com/logx"
	"go.osspkg.com/syncing"
	"go.osspkg.com/xc"

	"go.osspkg.com/network/address"
)

type (
	ConfigTCP struct {
		Addr            string        `yaml:"addr"`
		ReadTimeout     time.Duration `yaml:"read_timeout,omitempty"`
		WriteTimeout    time.Duration `yaml:"write_timeout,omitempty"`
		IdleTimeout     time.Duration `yaml:"idle_timeout,omitempty"`
		ShutdownTimeout time.Duration `yaml:"shutdown_timeout,omitempty"`
		CountEvents     uint          `yaml:"count_events,omitempty"`
		WaitIntervalMS  uint          `yaml:"wait_interval_ms,omitempty"`
	}

	ServerTCP struct {
		wg       syncing.Group
		Handler  func(ctx context.Context, w io.Writer, r io.Reader) error
		Config   ConfigTCP
		listener net.Listener
		epoll    TEpoll
	}
)

func (s *ServerTCP) init() (err error) {
	if s.Handler == nil {
		return fmt.Errorf("epoll tcp: handler is empty")
	}
	s.wg = syncing.NewGroup()
	s.Config.Addr = address.ResolveIPPort(s.Config.Addr)
	if s.Config.CountEvents == 0 {
		s.Config.CountEvents = 100
	}
	if s.Config.WaitIntervalMS == 0 {
		s.Config.WaitIntervalMS = 500
	}
	s.epoll, err = New(Option{
		Handler:        s.Handler,
		CountEvents:    s.Config.CountEvents,
		WaitIntervalMS: s.Config.WaitIntervalMS,
	})
	return
}

func (s *ServerTCP) ListenAndServe(ctx xc.Context) (err error) {
	defer func() {
		ctx.Close()
		logx.Error("Epoll server stopped", "err", err, "ip", s.Config.Addr)
	}()

	if err = s.init(); err != nil {
		return
	}
	if s.listener, err = net.Listen("tcp", s.Config.Addr); err != nil {
		return
	}
	defer func() {
		err = errors.Wrap(err, s.listener.Close())
	}()
	s.wg.Background(func() {
		s.connAccept(ctx)
	})
	s.wg.Background(func() {
		s.epollListen(ctx)
	})
	logx.Info("Epoll server started", "ip", s.Config.Addr)
	s.wg.Wait()
	return
}

func (s *ServerTCP) connAccept(ctx xc.Context) {
	defer func() {
		ctx.Close()
	}()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				logx.Error("Epoll conn accept", "err", err)
				return
			}
		}
		if err = s.epoll.Accept(conn); err != nil {
			logx.Error("Epoll append connect", "err", err, "ip", conn.RemoteAddr())
		}
	}
}

func (s *ServerTCP) epollListen(ctx xc.Context) {
	defer func() {
		ctx.Close()
	}()

	if err := s.epoll.Listen(ctx.Context()); err != nil {
		logx.Error("Epoll listen connects", "err", err)
	}
}
