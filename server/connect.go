/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"go.osspkg.com/ioutils"
	"go.osspkg.com/ioutils/pool"
)

var (
	connPool = pool.New[*Connect](func() *Connect {
		return &Connect{}
	})
)

type (
	TAddress interface {
		Addr() string
	}

	Connect struct {
		timeout time.Duration
		conn    net.Conn
		buff    *bytes.Buffer
	}
)

func (v *Connect) Set(c net.Conn, t time.Duration) {
	v.timeout = t
	v.conn = c
}

func (v *Connect) Reset() {
	if v.buff == nil {
		v.buff = bytes.NewBuffer(make([]byte, 0, 512))
	}
	v.buff.Reset()
	v.conn = nil
}

func (v *Connect) validate() error {
	if v.conn == nil {
		return fmt.Errorf("net connect is empty")
	}
	if v.buff == nil {
		v.buff = bytes.NewBuffer(make([]byte, 0, 512))
	}
	if v.timeout == 0 {
		v.timeout = 3 * time.Second
	}
	return nil
}

func (v *Connect) Wait() error {
	if err := v.validate(); err != nil {
		return err
	}
	if _, err := ioutils.Copy(v.buff, v.conn); err != nil {
		return err
	}
	return nil
}

func (v *Connect) IsEmpty() bool {
	return v.buff.Len() == 0
}

func (v *Connect) Read(b []byte) (int, error) {
	return v.buff.Read(b)
}

func (v *Connect) Addr() string {
	return v.conn.RemoteAddr().String()
}

func (v *Connect) Write(b []byte) (int, error) {
	return v.conn.Write(b)
}
