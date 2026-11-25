//go:build linux

/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package epoll

import (
	"context"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"

	"go.osspkg.com/errors"
)

const (
	fdListener int32 = -1
	fdWake     int32 = -2
)

type server struct {
	conf Config
	opts options

	mu      sync.Mutex
	started bool
	closed  bool
	wake    int
	done    chan struct{}
	clients map[int]*Conn
	addr    net.Addr
}

// Conn is a client TCP connection managed by the epoll reactor.
type Conn struct {
	server *server
	fd     int
	addr   net.Addr

	mu      sync.Mutex
	out     []byte
	closing bool
	closed  bool
	lastIO  time.Time
}

// New validates the configuration and creates a Linux epoll server.
func New(c Config, opts ...Option) (Server, error) {
	optionsValue, err := validate(c, opts)
	if err != nil {
		return nil, err
	}
	if c.OnData == nil {
		return nil, errors.New("epoll OnData callback is required")
	}
	return &server{conf: c, opts: optionsValue, wake: -1, done: make(chan struct{}), clients: make(map[int]*Conn)}, nil
}

// Write queues bytes for transmission by the reactor. It copies data before
// returning and wakes the event loop when called from another goroutine.
func (c *Conn) Write(data []byte) error {
	c.mu.Lock()
	if c.closing || c.closed {
		c.mu.Unlock()
		return net.ErrClosed
	}
	c.out = append(c.out, data...)
	c.lastIO = time.Now()
	c.mu.Unlock()
	c.server.signal()
	return nil
}

// Close schedules this connection to be closed by the reactor.
func (c *Conn) Close() error {
	c.mu.Lock()
	if c.closing || c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closing = true
	c.mu.Unlock()
	c.server.signal()
	return nil
}

// RemoteAddr returns the peer's network address.
func (c *Conn) RemoteAddr() net.Addr { return c.addr }

func (s *server) ListenAndServe(ctx context.Context) error {
	if ctx == nil {
		return errors.New("epoll server context is nil")
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("epoll server is closed")
	}
	if s.started {
		s.mu.Unlock()
		return errors.New("epoll server is already running")
	}
	s.started = true
	s.mu.Unlock()
	defer close(s.done)

	if ctx.Err() != nil {
		return nil
	}
	listener, err := listenSocket(s.conf.Address)
	if err != nil {
		return errors.Wrapf(err, "listen on %q", s.conf.Address)
	}
	defer func() { _ = syscall.Close(listener) }()
	boundAddr, err := syscall.Getsockname(listener)
	if err != nil {
		return errors.Wrapf(err, "get listening address")
	}
	s.mu.Lock()
	s.addr = sockaddrToTCP(boundAddr)
	s.mu.Unlock()
	epfd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return errors.Wrapf(err, "create epoll instance")
	}
	defer func() { _ = syscall.Close(epfd) }()
	defer func() {
		for _, conn := range s.connections() {
			s.closeConnection(epfd, conn, net.ErrClosed)
		}
	}()
	wakePipe := [2]int{-1, -1}
	if err := syscall.Pipe2(wakePipe[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		return errors.Wrapf(err, "create reactor wake pipe")
	}
	defer func() { _ = syscall.Close(wakePipe[0]) }()
	defer func() { _ = syscall.Close(wakePipe[1]) }()
	s.mu.Lock()
	s.wake = wakePipe[1]
	closed := s.closed
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.wake = -1
		s.mu.Unlock()
	}()

	for fd, tag := range map[int]int32{listener: fdListener, wakePipe[0]: fdWake} {
		if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: tag}); err != nil {
			return errors.Wrapf(err, "register fd %d with epoll", fd)
		}
	}
	if closed {
		s.signal()
	}
	stopWake := context.AfterFunc(ctx, s.signal)
	defer stopWake()
	events := make([]syscall.EpollEvent, 128)
	for {
		if ctx.Err() != nil || s.isClosed() {
			return nil
		}
		timeout := s.pollTimeout()
		n, err := syscall.EpollWait(epfd, events, timeout)
		if err != nil && err != syscall.EINTR {
			return errors.Wrapf(err, "wait for epoll events")
		}
		for i := 0; i < n; i++ {
			event := events[i]
			switch event.Fd {
			case fdListener:
				if err := s.acceptReady(epfd, listener); err != nil {
					return err
				}
			case fdWake:
				drainFD(wakePipe[0])
			default:
				if conn := s.connection(int(event.Fd)); conn != nil {
					s.handleReady(epfd, conn, event.Events)
				}
			}
		}
		s.expireIdle(epfd)
		for _, conn := range s.connections() {
			if err := s.updateInterest(epfd, conn); err != nil {
				s.closeConnection(epfd, conn, err)
			}
		}
	}
}

func (s *server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	s.signal()
	return nil
}

// Addr returns the bound TCP address after ListenAndServe has opened the socket.
func (s *server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

func (s *server) signal() {
	s.mu.Lock()
	wake := s.wake
	s.mu.Unlock()
	if wake >= 0 {
		_, _ = syscall.Write(wake, []byte{1})
	}
}

func (s *server) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func listenSocket(address string) (int, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return -1, err
	}
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, syscall.IPPROTO_TCP)
	if err != nil {
		return -1, err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = syscall.Close(fd)
		}
	}()
	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return -1, err
	}
	var sa syscall.Sockaddr
	if ip4 := tcpAddr.IP.To4(); ip4 != nil || tcpAddr.IP == nil {
		addr := &syscall.SockaddrInet4{Port: tcpAddr.Port}
		if ip4 != nil {
			copy(addr.Addr[:], ip4)
		}
		sa = addr
	} else {
		addr := &syscall.SockaddrInet6{Port: tcpAddr.Port}
		copy(addr.Addr[:], tcpAddr.IP.To16())
		sa = addr
	}
	if err = syscall.Bind(fd, sa); err != nil {
		return -1, err
	}
	if err = syscall.Listen(fd, syscall.SOMAXCONN); err != nil {
		return -1, err
	}
	closeOnError = false
	return fd, nil
}

func (s *server) acceptReady(epfd, listener int) error {
	for {
		if s.isClosed() {
			return nil
		}
		fd, sa, err := syscall.Accept4(listener, syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC)
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			return nil
		}
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "accept TCP connection")
		}
		if len(s.clients) >= s.opts.maxConnections {
			_ = syscall.Close(fd)
			continue
		}
		conn := &Conn{server: s, fd: fd, addr: sockaddrToTCP(sa), lastIO: time.Now()}
		s.clients[fd] = conn
		if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{Events: syscall.EPOLLIN | syscall.EPOLLRDHUP, Fd: int32(fd)}); err != nil {
			s.closeConnection(epfd, conn, err)
			continue
		}
		if s.conf.OnConnect != nil {
			if err := s.conf.OnConnect(conn); err != nil {
				s.closeConnection(epfd, conn, err)
				continue
			}
		}
		if conn.isClosing() {
			s.closeConnection(epfd, conn, nil)
		}
	}
}

func (s *server) handleReady(epfd int, conn *Conn, events uint32) {
	if events&syscall.EPOLLERR != 0 {
		err := socketError(conn.fd)
		s.closeConnection(epfd, conn, err)
		return
	}
	if events&syscall.EPOLLIN != 0 {
		for {
			buffer := make([]byte, 32*1024)
			n, err := syscall.Read(conn.fd, buffer)
			if err == syscall.EINTR {
				continue
			}
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				break
			}
			if err != nil {
				s.closeConnection(epfd, conn, err)
				return
			}
			if n == 0 {
				s.closeConnection(epfd, conn, nil)
				return
			}
			conn.mu.Lock()
			conn.lastIO = time.Now()
			conn.mu.Unlock()
			data := append([]byte(nil), buffer[:n]...)
			if err := s.conf.OnData(conn, data); err != nil {
				s.closeConnection(epfd, conn, err)
				return
			}
			if conn.isClosing() {
				s.closeConnection(epfd, conn, nil)
				return
			}
		}
	}
	if events&syscall.EPOLLOUT != 0 {
		if err := s.flush(conn); err != nil {
			s.closeConnection(epfd, conn, err)
		}
	}
	if events&(syscall.EPOLLHUP|syscall.EPOLLRDHUP) != 0 {
		s.closeConnection(epfd, conn, nil)
	}
	return
}

func (s *server) flush(conn *Conn) error {
	for {
		conn.mu.Lock()
		if len(conn.out) == 0 {
			conn.mu.Unlock()
			return nil
		}
		n, err := syscall.Write(conn.fd, conn.out)
		if n > 0 {
			conn.out = conn.out[n:]
			conn.lastIO = time.Now()
		}
		conn.mu.Unlock()
		if err == syscall.EINTR {
			continue
		}
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			return nil
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return syscall.EPIPE
		}
	}
}

func (s *server) updateInterest(epfd int, conn *Conn) error {
	if conn.isClosing() {
		s.closeConnection(epfd, conn, nil)
		return nil
	}
	conn.mu.Lock()
	writable := len(conn.out) > 0
	conn.mu.Unlock()
	events := uint32(syscall.EPOLLIN | syscall.EPOLLRDHUP)
	if writable {
		events |= syscall.EPOLLOUT
	}
	return syscall.EpollCtl(epfd, syscall.EPOLL_CTL_MOD, conn.fd, &syscall.EpollEvent{Events: events, Fd: int32(conn.fd)})
}

func (s *server) closeConnection(epfd int, conn *Conn, cause error) {
	if conn == nil {
		return
	}
	conn.mu.Lock()
	if conn.closed {
		conn.mu.Unlock()
		return
	}
	conn.closed = true
	conn.mu.Unlock()
	delete(s.clients, conn.fd)
	_ = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, conn.fd, nil)
	_ = syscall.Close(conn.fd)
	if s.conf.OnClose != nil {
		s.conf.OnClose(conn, cause)
	}
}

func (s *server) connection(fd int) *Conn { return s.clients[fd] }

func (s *server) connections() []*Conn {
	result := make([]*Conn, 0, len(s.clients))
	for _, conn := range s.clients {
		result = append(result, conn)
	}
	return result
}

func (s *server) expireIdle(epfd int) {
	if s.opts.idleTimeout <= 0 {
		return
	}
	cutoff := time.Now().Add(-s.opts.idleTimeout)
	for _, conn := range s.connections() {
		conn.mu.Lock()
		expired := conn.lastIO.Before(cutoff)
		conn.mu.Unlock()
		if expired {
			s.closeConnection(epfd, conn, fmt.Errorf("connection idle timeout"))
		}
	}
}

func (s *server) pollTimeout() int {
	if s.opts.idleTimeout <= 0 {
		return -1
	}
	return max(1, min(1000, int(s.opts.idleTimeout/time.Millisecond)))
}

func (c *Conn) isClosing() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closing || c.closed
}

func drainFD(fd int) {
	var buf [128]byte
	for {
		_, err := syscall.Read(fd, buf[:])
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK || err != nil {
			return
		}
	}
}

func socketError(fd int) error {
	value, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
	if err != nil {
		return err
	}
	if value == 0 {
		return nil
	}
	return syscall.Errno(value)
}

func sockaddrToTCP(sa syscall.Sockaddr) net.Addr {
	switch addr := sa.(type) {
	case *syscall.SockaddrInet4:
		return &net.TCPAddr{IP: net.IP(addr.Addr[:]), Port: addr.Port}
	case *syscall.SockaddrInet6:
		return &net.TCPAddr{IP: net.IP(addr.Addr[:]), Port: addr.Port, Zone: zoneName(addr.ZoneId)}
	default:
		return &net.TCPAddr{}
	}
}

func zoneName(id uint32) string {
	if id == 0 {
		return ""
	}
	iface, err := net.InterfaceByIndex(int(id))
	if err != nil {
		return ""
	}
	return iface.Name
}
