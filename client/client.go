/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"go.osspkg.com/algorithms/control"
	"go.osspkg.com/errors"
	"go.osspkg.com/ioutils"
	"go.osspkg.com/network/internal"
)

type Client struct {
	Network     string
	Address     string
	Certificate *Certificate

	Timeout   time.Duration
	KeepAlive time.Duration

	MaxIdleConns int
	BufferSize   int

	err    error
	config *tls.Config
	pool   *chanPool[*connect]
	sem    control.Semaphore
	once   sync.Once
}

func (v *Client) setup() error {
	v.once.Do(func() {
		if v.err = internal.IsPassableNetwork(v.Network); v.err != nil {
			return
		}

		var (
			err  error
			addr fmt.Stringer
		)

		if err = v.applyTLSCertificate(); err != nil {
			v.err = err
			return
		}

		switch v.Network {
		case internal.NetTCP:
			addr, err = net.ResolveTCPAddr(v.Network, v.Address)
		case internal.NetUDP:
			addr, err = net.ResolveUDPAddr(v.Network, v.Address)
		case internal.NetQUIC:
			if len(v.config.NextProtos) == 0 {
				v.config.NextProtos = append(v.config.NextProtos, "quic")
			}
			addr, err = net.ResolveUDPAddr(internal.NetUDP, v.Address)
		case internal.NetUNIX:
			addr, err = net.ResolveUnixAddr(v.Network, v.Address)
		default:
			addr, err = nil, fmt.Errorf("invalid network name, use: tcp, udp, unix, quic")
		}
		if err != nil {
			v.err = err
			return
		}

		v.BufferSize = internal.NotZero[int](v.BufferSize, 65535)
		v.Timeout = internal.NotZeroDuration(v.Timeout, 1*time.Second)
		v.KeepAlive = internal.NotZeroDuration(v.KeepAlive, 15*time.Second)
		v.MaxIdleConns = internal.NotZero[int](v.MaxIdleConns, 1)

		v.Address = addr.String()
		v.sem = control.NewSemaphore(uint64(v.MaxIdleConns))

		v.pool = newChanPool[*connect](v.MaxIdleConns, func() *connect {
			pIdleAt := time.Now().Add(v.KeepAlive)
			pconn, pclose, perr := v.dialConnect(context.Background())
			return &connect{
				Conn:      pconn,
				CloseFunc: pclose,
				Err:       perr,
				IdleAt:    pIdleAt,
			}
		})
	})

	if v.err != nil {
		return v.err
	}
	return nil
}

func (v *Client) applyTLSCertificate() error {
	if v.Certificate == nil {
		return nil
	}
	if v.config == nil {
		v.config = internal.DefaultTLSConfig()
	}
	cert, ca, err := parseCertificate(*v.Certificate)
	if err != nil {
		return err
	}
	if ca != nil {
		v.config.RootCAs = ca
	}
	if len(cert.Certificate) >= 0 {
		v.config.Certificates = append(v.config.Certificates, cert)
	}
	v.config.InsecureSkipVerify = v.Certificate.InsecureSkipVerify

	return nil
}

func (v *Client) dialConnect(ctx context.Context) (action, func(), error) {
	if v.Network == internal.NetQUIC {
		conn, err := quic.DialAddr(ctx, v.Address, v.config, &quic.Config{EnableDatagrams: true})
		if err != nil {
			return nil, nil, fmt.Errorf("create connect: %w", err)
		}

		stream, err := conn.OpenStream()
		if err != nil {
			writeLog(conn.CloseWithError(0, ""), "close connect", v.Network, v.Address)
			return nil, nil, fmt.Errorf("open stream: %w", err)
		}

		return stream, func() {
			writeLog(stream.Close(), "close stream", v.Network, v.Address)
			writeLog(conn.CloseWithError(0, ""), "close connect", v.Network, v.Address)
		}, nil
	}

	if v.Certificate != nil {
		dial := &tls.Dialer{
			NetDialer: new(net.Dialer),
			Config:    v.config,
		}
		conn, err := dial.DialContext(ctx, v.Network, v.Address)
		if err != nil {
			return nil, nil, fmt.Errorf("create connect: %w", err)
		}
		return conn, func() {
			writeLog(conn.Close(), "close connect", v.Network, v.Address)
		}, nil
	}

	var dial net.Dialer
	conn, err := dial.DialContext(ctx, v.Network, v.Address)
	if err != nil {
		return nil, nil, fmt.Errorf("create connect: %w", err)
	}
	return conn, func() {
		writeLog(conn.Close(), "close connect", v.Network, v.Address)
	}, nil
}

func (v *Client) Do(in io.Reader, out io.Writer) (err error) {
	if err = v.setup(); err != nil {
		return
	}

	v.sem.Acquire()
	defer func() { v.sem.Release() }()

	conn := v.pool.GetIdleOrCreateConn()

	if err = conn.GetError(); err != nil {
		return
	}

	defer func() {
		conn.Err = errors.Wrap(conn.Err, err)
		v.pool.PutOrCloseIdleConn(conn)
	}()

	errC := make(chan error, 1)
	startC := make(chan struct{})

	go func() {
		close(startC)

		if e := internal.Deadline(conn.Conn, v.Timeout*2); e != nil {
			errC <- fmt.Errorf("update deadline: %w", e)
			return
		}

		n, e := ioutils.CopyPack(out, conn.Conn, v.BufferSize)
		if e != nil {
			errC <- fmt.Errorf("read message: %w", e)
			return
		} else if n == 0 {
			errC <- fmt.Errorf("read message: got 0 bytes")
			return
		}

		errC <- nil
	}()

	<-startC

	n, e := ioutils.CopyPack(conn.Conn, in, v.BufferSize)
	if e != nil {
		err = fmt.Errorf("write message: %w", e)
		return
	} else if n == 0 {
		err = fmt.Errorf("write message: set 0 bytes")
		return
	}

	if err = <-errC; err != nil {
		return
	}

	if err = conn.Conn.SetWriteDeadline(time.Now().Add(v.KeepAlive)); err != nil {
		err = fmt.Errorf("update deadline: %w", err)
		return
	}

	return
}
