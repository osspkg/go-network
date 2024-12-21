/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package epoll

import (
	"context"
	"fmt"
	"net"
	"sync"
	"syscall"

	"go.osspkg.com/do"
	"go.osspkg.com/errors"
	"go.osspkg.com/ioutils"
	"go.osspkg.com/logx"
	netfd "go.osspkg.com/network/fd"
	"golang.org/x/sys/unix"
)

type (
	_epoll struct {
		fd     int
		pipe   chan TConnect
		conn   map[int32]TConnect
		events []unix.EpollEvent
		cfg    Option
		mux    sync.RWMutex
	}
	TEpoll interface {
		Accept(c net.Conn) error
		Listen(ctx context.Context) (err error)
	}
)

func New(c Option) (TEpoll, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	v, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &_epoll{
		fd:     v,
		cfg:    c,
		pipe:   make(chan TConnect, c.CountEvents),
		conn:   make(map[int32]TConnect, c.CountEvents),
		events: make([]unix.EpollEvent, c.WaitIntervalMS),
	}, nil
}

func (v *_epoll) Accept(c net.Conn) error {
	fd64 := netfd.ByConnect(c)
	fd32 := int32(fd64)
	err := unix.EpollCtl(v.fd, syscall.EPOLL_CTL_ADD, int(fd64), &unix.EpollEvent{Events: epollEvents, Fd: fd32})
	if err != nil {
		return errors.Wrap(err, c.Close())
	}
	v.mux.Lock()
	v.conn[fd32] = newConnect(c, fd32)
	v.mux.Unlock()
	return nil
}

func (v *_epoll) removeFD(fd int32) error {
	return unix.EpollCtl(v.fd, syscall.EPOLL_CTL_DEL, int(fd), nil)
}

func (v *_epoll) getConn(fd int32) (TConnect, bool) {
	v.mux.Lock()
	defer v.mux.Unlock()

	conn, ok := v.conn[fd]
	delete(v.conn, fd)
	return conn, ok
}

func (v *_epoll) setConn(c TConnect) {
	v.mux.Lock()
	defer v.mux.Unlock()

	v.conn[c.FD()] = c
}

func (v *_epoll) closeConn(fd int32) error {
	v.mux.Lock()
	defer v.mux.Unlock()

	conn, ok := v.conn[fd]
	if !ok {
		return nil
	}

	delete(v.conn, fd)

	return errors.Wrap(
		v.removeFD(fd),
		conn.Conn().Close(),
	)
}

func (v *_epoll) closeAll() (err error) {
	v.mux.Lock()
	defer v.mux.Unlock()

	for c := range v.pipe {
		if err0 := v.closeConn(c.FD()); err0 != nil {
			err = errors.Wrap(err, err0)
		}
	}

	for fd := range v.conn {
		if err0 := v.closeConn(fd); err0 != nil {
			err = errors.Wrap(err, err0)
		}
	}
	return
}

func (v *_epoll) getWaited(list *[]int32) (int, error) {
	n, err := unix.EpollWait(v.fd, v.events, int(v.cfg.WaitIntervalMS))
	if err != nil && !errors.Is(err, unix.EINTR) {
		return 0, err
	}
	if n <= 0 {
		return 0, nil
	}
	for i := 0; i < n; i++ {
		switch v.events[i].Events {
		case unix.POLLIN:
			*list = append(*list, v.events[i].Fd)
		default:
			if err = v.closeConn(v.events[i].Fd); err != nil {
				logx.Error("Epoll close connect", "err", err)
			}
		}
	}

	return len(*list), nil
}

func (v *_epoll) Listen(ctx context.Context) (err error) {
	defer func() {
		close(v.pipe)
		err = errors.Wrap(err, v.closeAll())
	}()

	go v.piping(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		list := connPool.Get()
		n, err0 := v.getWaited(&list.B)
		if err0 != nil {
			err = err0
			return
		}
		if n <= 0 {
			continue
		}

		for _, fd := range list.B {
			conn, ok := v.getConn(fd)
			if !ok {
				continue
			}
			v.pipe <- conn
		}

		connPool.Put(list)
	}

}

func (v *_epoll) piping(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case conn := <-v.pipe:
			do.Async(func() {
				defer func() {
					v.setConn(conn)
				}()

				e := v.handlingConnect(ctx, conn)
				if e == nil {
					return
				}
				if !isClosedError(e) {
					logx.Warn("Epoll handling connect", "err", e, "ip", conn.Conn().RemoteAddr())
					return
				}
				e = v.closeConn(conn.FD())
				if e == nil || isClosedError(e) {
					return
				}
				logx.Error("Epoll close connect", "err", e)
			}, func(e error) {
				logx.Error("Epoll pipe panic", "err", errors.Unwrap(e), "full", e)
			})
		}
	}
}

func (v *_epoll) handlingConnect(ctx context.Context, conn TConnect) error {
	buff := buffPool.Get()
	defer func() {
		buffPool.Put(buff)
	}()
	n, err := ioutils.Copy(buff, conn.Conn())
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	return v.cfg.Handler(context.WithoutCancel(ctx), conn.Conn(), buff)
}
