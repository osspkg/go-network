/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import (
	"fmt"
	"time"

	"go.osspkg.com/errors"
)

const (
	NetTCP  = "tcp"
	NetUDP  = "udp"
	NetUNIX = "unix"
	NetQUIC = "quic"
)

func IsPassableNetwork(network string) error {
	switch network {
	case NetTCP, NetUDP, NetUNIX, NetQUIC:
		return nil
	default:
		return fmt.Errorf("invalid network type, use: tcp, udp, unix, quic")
	}
}

type TDeadline interface {
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

func Deadline(c TDeadline, ttl time.Duration) error {
	t := time.Now().Add(ttl)
	return errors.Wrap(
		c.SetReadDeadline(t),
		c.SetWriteDeadline(t),
	)
}
