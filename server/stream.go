/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"time"

	"go.osspkg.com/errors"
)

func (v *object) serveStreams(ctx context.Context, listener net.Listener) error {
	slots := make(chan struct{}, v.maxConnections)
	for {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			return nil
		}

		conn, err := listener.Accept()
		if err != nil {
			<-slots
			if ctx.Err() != nil || v.isClosed() {
				return err
			}
			return errors.Wrapf(err, "accept connection")
		}
		if ctx.Err() != nil || !v.track(conn) {
			_ = conn.Close()
			<-slots
			return net.ErrClosed
		}

		v.workers.Add(1)
		go v.handleStream(ctx, conn, slots)
	}
}

func (v *object) handleStream(ctx context.Context, conn net.Conn, slots chan struct{}) {
	defer v.workers.Done()
	defer func() {
		v.untrack(conn)
		_ = conn.Close()
		<-slots
	}()

	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopClose := context.AfterFunc(connCtx, func() { _ = conn.Close() })
	defer stopClose()

	if tlsConn, ok := conn.(*tls.Conn); ok {
		if v.idleTimeout > 0 {
			if err := conn.SetDeadline(time.Now().Add(v.idleTimeout)); err != nil {
				log.Printf("network server: set TLS handshake deadline for %s failed: %v", conn.RemoteAddr(), err)
				return
			}
		}
		if err := tlsConn.HandshakeContext(connCtx); err != nil {
			if connCtx.Err() == nil {
				log.Printf("network server: TLS handshake from %s failed: %v", conn.RemoteAddr(), err)
			}
			return
		}
		if v.idleTimeout > 0 {
			if err := conn.SetDeadline(time.Time{}); err != nil {
				log.Printf("network server: clear TLS handshake deadline for %s failed: %v", conn.RemoteAddr(), err)
				return
			}
		}
	}

	handlerConn := net.Conn(conn)
	if v.idleTimeout > 0 {
		idleConn := newIdleConn(conn, v.idleTimeout, func() {
			cancel()
			_ = conn.Close()
		})
		handlerConn = idleConn
		defer idleConn.Close() //nolint:errcheck
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("network server: handler panic from %s: %v", conn.RemoteAddr(), recovered)
		}
	}()
	if err := v.handler(connCtx, handlerConn, conn.RemoteAddr()); err != nil && connCtx.Err() == nil {
		log.Printf("network server: handler from %s failed: %v", conn.RemoteAddr(), err)
	}
}

func (v *object) track(conn net.Conn) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return false
	}
	v.active[conn] = struct{}{}
	return true
}

func (v *object) untrack(conn net.Conn) {
	v.mu.Lock()
	delete(v.active, conn)
	v.mu.Unlock()
}
