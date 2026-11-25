/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.osspkg.com/errors"
)

var ErrClientClosed = errors.New("client is closed")

type pooledConn struct {
	conn      net.Conn
	idleSince time.Time
	idle      bool
	timer     *time.Timer
	closed    atomic.Bool
}

type connPool struct {
	dial        func(context.Context) (net.Conn, error)
	idleTimeout time.Duration
	slots       chan struct{}
	idle        chan *pooledConn
	ctx         context.Context
	cancel      context.CancelFunc

	mu     sync.Mutex
	closed bool
	all    map[*pooledConn]struct{}
}

func newConnPool(size int, idleTimeout time.Duration, dial func(context.Context) (net.Conn, error)) *connPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &connPool{
		dial:        dial,
		idleTimeout: idleTimeout,
		slots:       make(chan struct{}, size),
		idle:        make(chan *pooledConn, size),
		ctx:         ctx,
		cancel:      cancel,
		all:         make(map[*pooledConn]struct{}),
	}
}

func (p *connPool) acquire(ctx context.Context) (*pooledConn, error) {
	for {
		if p.isClosed() {
			return nil, ErrClientClosed
		}
		if conn := p.takeIdle(); conn != nil {
			if p.checkout(conn) {
				return conn, nil
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-p.ctx.Done():
			return nil, ErrClientClosed
		case conn := <-p.idle:
			if p.checkout(conn) {
				return conn, nil
			}
		case p.slots <- struct{}{}:
			dialCtx, cancel := context.WithCancel(ctx)
			stopClose := context.AfterFunc(p.ctx, cancel)
			conn, err := p.dial(dialCtx)
			stopClose()
			cancel()
			if err != nil {
				<-p.slots
				return nil, err
			}
			pooled := &pooledConn{conn: conn}

			p.mu.Lock()
			if p.closed {
				p.mu.Unlock()
				_ = conn.Close()
				<-p.slots
				return nil, ErrClientClosed
			}
			p.all[pooled] = struct{}{}
			p.mu.Unlock()
			return pooled, nil
		}
	}
}

func (p *connPool) takeIdle() *pooledConn {
	select {
	case conn := <-p.idle:
		return conn
	default:
		return nil
	}
}

func (p *connPool) checkout(conn *pooledConn) bool {
	p.mu.Lock()
	if p.closed || conn.closed.Load() || !conn.idle {
		p.mu.Unlock()
		return false
	}
	if time.Since(conn.idleSince) >= p.idleTimeout {
		conn.idle = false
		if conn.timer != nil {
			conn.timer.Stop()
			conn.timer = nil
		}
		p.mu.Unlock()
		p.drop(conn)
		return false
	}
	conn.idle = false
	if conn.timer != nil {
		conn.timer.Stop()
		conn.timer = nil
	}
	p.mu.Unlock()
	return true
}

func (p *connPool) release(conn *pooledConn, reusable bool) {
	if !reusable || conn.closed.Load() {
		p.drop(conn)
		return
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		p.drop(conn)
		return
	}
	conn.idleSince = time.Now()
	conn.idle = true
	select {
	case p.idle <- conn:
		conn.timer = time.AfterFunc(p.idleTimeout, func() { p.expireIdle(conn) })
		p.mu.Unlock()
	default:
		conn.idle = false
		p.mu.Unlock()
		p.drop(conn)
	}
}

func (p *connPool) expireIdle(conn *pooledConn) {
	p.mu.Lock()
	if p.closed || !conn.idle || conn.closed.Load() {
		p.mu.Unlock()
		return
	}
	conn.idle = false
	conn.timer = nil
	p.mu.Unlock()
	p.drop(conn)
}

func (p *connPool) drop(conn *pooledConn) {
	_ = p.closeConn(conn)
}

func (p *connPool) closeConn(conn *pooledConn) error {
	if !conn.closed.CompareAndSwap(false, true) {
		return nil
	}
	err := conn.conn.Close()

	p.mu.Lock()
	conn.idle = false
	if conn.timer != nil {
		conn.timer.Stop()
		conn.timer = nil
	}
	_, registered := p.all[conn]
	delete(p.all, conn)
	p.mu.Unlock()
	if registered {
		<-p.slots
	}
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (p *connPool) isClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

func (p *connPool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.cancel()
	connections := make([]*pooledConn, 0, len(p.all))
	for conn := range p.all {
		connections = append(connections, conn)
	}
	p.mu.Unlock()

	var closeErrors []error
	for _, conn := range connections {
		if err := p.closeConn(conn); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	for {
		select {
		case <-p.idle:
		default:
			return errors.Wrap(closeErrors...)
		}
	}
}
