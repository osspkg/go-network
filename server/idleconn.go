/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"net"
	"sync"
	"time"
)

type idleConn struct {
	net.Conn
	timeout time.Duration
	onIdle  func()
	timer   *time.Timer
	mu      sync.Mutex
	closed  bool
	lastIO  time.Time
}

func newIdleConn(conn net.Conn, timeout time.Duration, onIdle func()) *idleConn {
	v := &idleConn{Conn: conn, timeout: timeout, onIdle: onIdle, lastIO: time.Now()}
	v.timer = time.AfterFunc(timeout, v.expire)
	return v
}

func (v *idleConn) Read(p []byte) (int, error) {
	n, err := v.Conn.Read(p)
	if n > 0 {
		v.reset()
	}
	return n, err
}

func (v *idleConn) Write(p []byte) (int, error) {
	n, err := v.Conn.Write(p)
	if n > 0 {
		v.reset()
	}
	return n, err
}

func (v *idleConn) Close() error {
	v.mu.Lock()
	if !v.closed {
		v.closed = true
		v.timer.Stop()
	}
	v.mu.Unlock()
	return v.Conn.Close()
}

func (v *idleConn) reset() {
	v.mu.Lock()
	if !v.closed {
		v.lastIO = time.Now()
		v.timer.Reset(v.timeout)
	}
	v.mu.Unlock()
}

func (v *idleConn) expire() {
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return
	}
	if remaining := v.timeout - time.Since(v.lastIO); remaining > 0 {
		v.timer.Reset(remaining)
		v.mu.Unlock()
		return
	}
	v.closed = true
	v.mu.Unlock()
	v.onIdle()
}
