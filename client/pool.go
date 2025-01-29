/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"fmt"
	"io"
	"time"

	"go.osspkg.com/network/internal"
)

type action interface {
	io.Reader
	io.Writer
	internal.TDeadline
}

type (
	connect struct {
		conn       action
		closeFunc  func()
		err        error
		idleAt     time.Time
		keepAlive  time.Duration
		timeOut    time.Duration
		bufferSize int
	}

	Pipe interface {
		Dispatch(r io.Reader) error
		Receive(w io.Writer) error
	}
)

func (c *connect) Dispatch(r io.Reader) error {
	//if err := internal.Deadline(c.conn, c.timeOut); err != nil {
	//	return err
	//}

	//n, err := ioutils.CopyN(c.conn, r, c.bufferSize)
	n, err := io.Copy(c.conn, r)
	if err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("write message: set 0 bytes")
	}

	//return internal.Deadline(c.conn, c.keepAlive)
	return nil
}

func (c *connect) Receive(w io.Writer) error {
	//if err := internal.Deadline(c.conn, c.timeOut); err != nil {
	//	return err
	//}

	//n, err := ioutils.CopyN(w, c.conn, c.bufferSize)
	n, err := io.Copy(w, c.conn)
	if err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("read message: set 0 bytes")
	}

	//return internal.Deadline(c.conn, c.keepAlive)
	return nil
}

func (c *connect) Close() {
	if c.closeFunc != nil {
		c.closeFunc()
	}
}

func (c *connect) IsFailConn() bool {
	return c.err != nil || time.Now().After(c.idleAt)
}

func (c *connect) GetError() error {
	return c.err
}

type (
	connPool struct {
		connCh  chan *connect
		callNew func() *connect
	}
)

func newConnPool(size int, call func() *connect) *connPool {
	return &connPool{
		connCh:  make(chan *connect, size+1),
		callNew: call,
	}
}

func (p *connPool) Get() (v *connect) {
	var isNew bool
	for {
		select {
		case v = <-p.connCh:
		default:
			v = p.callNew()
			isNew = true
		}

		if !isNew && v.IsFailConn() {
			v.Close()
			continue
		}

		v.idleAt = time.Now().Add(v.keepAlive)

		return
	}
}

func (p *connPool) Put(v *connect) {
	if v.IsFailConn() {
		v.Close()
		return
	}

	select {
	case p.connCh <- v:
		return
	default:
		v.Close()
	}
}
