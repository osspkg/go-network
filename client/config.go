/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"fmt"
	"net"

	"go.osspkg.com/network/cert"
	"go.osspkg.com/network/xnet"
)

type Config struct {
	Address     string             `yaml:"address" unic:"address,default='127.0.0.1:80',desc='Адрес сервера'"`
	Network     string             `yaml:"network" unic:"network,default='tcp',desc='Тип сетевого протокола (tcp,udp,unix)'"`
	Certificate *cert.ClientConfig `yaml:"certificate,omitempty" unic:"certificate,omitempty,desc='Настройки шифрования соединения'"`
}

func (c Config) Resolve() (fmt.Stringer, error) {
	if err := xnet.IsPassableNetwork(c.Network); err != nil {
		return nil, err
	}

	switch c.Network {
	case xnet.NetTCP:
		return net.ResolveTCPAddr(xnet.NetTCP, c.Address)
	case xnet.NetUDP:
		return net.ResolveUDPAddr(xnet.NetUDP, c.Address)
	case xnet.NetUNIX:
		return net.ResolveUnixAddr(xnet.NetUNIX, c.Address)
	default:
		return nil, fmt.Errorf("invalid network name, use: tcp, udp, unix")
	}
}
