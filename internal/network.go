/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import (
	"fmt"
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
