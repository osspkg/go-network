/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"go.osspkg.com/errors"

	"go.osspkg.com/network/listen"
	"go.osspkg.com/network/xnet"
)

type object struct {
	conf           Config
	tls            *tls.Config
	maxConnections int
	idleTimeout    time.Duration

	mu       sync.Mutex
	handler  Handler
	started  bool
	closed   bool
	listener io.Closer
	cancel   context.CancelFunc
	active   map[net.Conn]struct{}
	unixInfo os.FileInfo
	workers  sync.WaitGroup
}

// New validates the configuration and creates a server. The Config fields are
// used as-is; resource limits can be set with options.
func New(c Config, options ...Option) (Server, error) {
	if err := xnet.IsPassableNetwork(c.Network); err != nil {
		return nil, err
	}
	if c.Address == "" {
		return nil, errors.New("server address is empty")
	}
	switch c.Network {
	case xnet.NetTCP, xnet.NetUDP:
		if _, _, err := net.SplitHostPort(c.Address); err != nil {
			return nil, errors.Wrapf(err, "invalid %s address %q", c.Network, c.Address)
		}
	case xnet.NetUNIX:
		if _, err := net.ResolveUnixAddr(xnet.NetUNIX, c.Address); err != nil {
			return nil, errors.Wrapf(err, "invalid unix address %q", c.Address)
		}
	}

	opts := defaultOptions()
	for _, option := range options {
		if option == nil {
			return nil, errors.New("server option is nil")
		}
		if err := option(&opts); err != nil {
			return nil, errors.Wrapf(err, "invalid server option")
		}
	}

	var tlsConfig *tls.Config
	if c.Certificate != nil {
		if c.Network != xnet.NetTCP {
			return nil, errors.New("TLS certificate configuration is only supported with TCP")
		}
		var err error
		tlsConfig, err = c.Certificate.Parse()
		if err != nil {
			return nil, errors.Wrapf(err, "fail parse certificate")
		}
	}
	return &object{
		conf:           c,
		tls:            tlsConfig,
		maxConnections: opts.maxConnections,
		idleTimeout:    opts.idleTimeout,
		active:         make(map[net.Conn]struct{}),
	}, nil
}

// HandleFunc registers the server handler. It must be called before
// ListenAndServe; later calls are ignored.
func (v *object) HandleFunc(handler Handler) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.started && !v.closed {
		v.handler = handler
	}
}

// ListenAndServe starts the configured transport and runs until the context is
// cancelled, Close is called, or the listener fails.
func (v *object) ListenAndServe(ctx context.Context) (serveErr error) {
	if ctx == nil {
		return errors.New("server context is nil")
	}

	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return errors.New("server is closed")
	}
	if v.started {
		v.mu.Unlock()
		return errors.New("server is already running")
	}
	if v.handler == nil {
		v.mu.Unlock()
		return errors.New("server handler is not configured")
	}
	v.started = true
	serveCtx, cancel := context.WithCancel(ctx)
	v.cancel = cancel
	v.mu.Unlock()

	defer func() {
		cancel()
		_ = v.Close()
		v.workers.Wait()
		v.removeUnixSocket()
	}()
	if serveCtx.Err() != nil {
		return nil
	}

	if v.conf.Network == xnet.NetUNIX {
		if err := cleanStaleUnixSocket(v.conf.Address); err != nil {
			return err
		}
	}

	listener, err := listen.New(serveCtx, v.conf.Network, v.conf.Address, v.tls)
	if err != nil {
		if serveCtx.Err() != nil {
			return nil
		}
		return errors.Wrapf(err, "create %s listener at %q", v.conf.Network, v.conf.Address)
	}

	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		_ = listener.Close()
		return nil
	}
	v.listener = listener
	if v.conf.Network == xnet.NetUNIX && !isAbstractUnix(v.conf.Address) {
		v.unixInfo, _ = os.Lstat(v.conf.Address)
	}
	v.mu.Unlock()

	stopContextClose := context.AfterFunc(serveCtx, func() { _ = v.Close() })
	defer stopContextClose()

	if streamListener, ok := listener.(net.Listener); ok {
		serveErr = v.serveStreams(serveCtx, streamListener)
	} else if packetConn, ok := listener.(net.PacketConn); ok {
		serveErr = v.serveDatagrams(serveCtx, packetConn)
	} else {
		serveErr = errors.New("listener does not support streams or datagrams")
	}

	if serveErr != nil && (serveCtx.Err() != nil || v.isClosed()) {
		return nil
	}
	return serveErr
}

// Close cancels active handlers and closes the listener and active stream
// connections. ListenAndServe waits for handlers to return before it exits.
func (v *object) Close() error {
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return nil
	}
	v.closed = true
	listener := v.listener
	cancel := v.cancel
	connections := make([]net.Conn, 0, len(v.active))
	for conn := range v.active {
		connections = append(connections, conn)
	}
	v.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	var closeErrs []error
	if listener != nil {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			closeErrs = append(closeErrs, err)
		}
	}
	for _, conn := range connections {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			closeErrs = append(closeErrs, err)
		}
	}
	return errors.Wrap(closeErrs...)
}

func (v *object) isClosed() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.closed
}
