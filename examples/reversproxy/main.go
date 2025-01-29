/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"

	"go.osspkg.com/logx"

	"go.osspkg.com/network/listen"
)

func main() {
	logx.SetLevel(logx.LevelDebug)

	//cli := &client.Client{
	//	Address: "imap.beget.com:993",
	//	//Address:      "imap.gmail.com:993",
	//	Network:      "tcp",
	//	KeepAlive:    5 * time.Minute,
	//	Timeout:      1 * time.Minute,
	//	MaxIdleConns: 1,
	//	Certificate: &client.Certificate{
	//		InsecureSkipVerify: false,
	//	},
	//}
	//
	//srv := server.New(server.Config{
	//	Address: "localhost:10993",
	//	Network: "tcp",
	//	SSL: &server.SSL{
	//		Certs: []listen.Certificate{
	//			{Addresses: []string{"localhost"}, AutoGenerate: true},
	//		},
	//		NextProtos: nil,
	//	},
	//})

	srv, err := listen.New(context.TODO(), "tcp", "localhost:10993", &listen.SSL{
		Certs: []listen.Certificate{
			{Addresses: []string{"localhost"}, AutoGenerate: true},
		},
	})
	if err != nil {
		panic(err)
	}

	l, ok := srv.(net.Listener)
	if !ok {
		panic("fail listener")
	}

	dial := &tls.Dialer{
		NetDialer: new(net.Dialer),
		Config: &tls.Config{
			MinVersion:         tls.VersionTLS10,
			Rand:               rand.Reader,
			ServerName:         "imap.beget.com",
			InsecureSkipVerify: true,
		},
	}

	cli, err := dial.DialContext(context.TODO(), "tcp", "imap.beget.com:993")
	if err != nil {
		panic(err)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			logx.Error("Failed to accept connection", "err", err)
			return
		}

		fmt.Println(conn.RemoteAddr())

		go pipe(cli, conn)
	}

}

const bufSize = 32 * 1024

func pipe(cli, conn net.Conn) {
	r := io.TeeReader(conn, &echo{Prefix: "REQ"})
	w := io.MultiWriter(conn, &echo{Prefix: "RES"})

	go func() {
		buf := make([]byte, bufSize)
		for {
			rn, re := r.Read(buf)
			fmt.Println(">>> send", rn, re)
			if rn > 0 {
				wn, we := cli.Write(buf[0:rn])
				if wn != rn || we != nil {
					logx.Error("SERV-CLI-1", "n", wn, "err", we)
					return
				}
			}
			if re != nil {
				logx.Error("SERV-CLI-2", "n", rn, "err", re)
				return
			}
		}
	}()

	buf := make([]byte, bufSize)
	for {
		rn, re := cli.Read(buf)
		fmt.Println(">>> recv", rn, re)
		if rn > 0 {
			wn, we := w.Write(buf[0:rn])
			if wn != rn || we != nil {
				logx.Error("CLI-SERV-1", "n", wn, "err", we)
				return
			}
		}
		if re != nil {
			logx.Error("CLI-SERV-2", "n", rn, "err", re)
			return
		}
	}
}

type echo struct {
	Prefix string
}

func (v *echo) Write(p []byte) (n int, err error) {
	fmt.Fprintf(os.Stdout, "[====== %s ======]\n", v.Prefix)
	return os.Stdout.Write(p)
}
