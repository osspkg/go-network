/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package xnet

import "go.osspkg.com/errors"

const (
	NetTCP  = "tcp"
	NetUDP  = "udp"
	NetUNIX = "unix"
)

func IsPassableNetwork(network string) error {
	switch network {
	case NetTCP, NetUDP, NetUNIX:
		return nil
	default:
		return errors.New("invalid network type, use: tcp, udp, unix")
	}
}
