/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package client

import (
	"go.osspkg.com/network/cert"
)

type Config struct {
	Address     string             `yaml:"address" unic:"address,default='127.0.0.1:80',desc='Server address'"`
	Network     string             `yaml:"network" unic:"network,default='tcp',desc='Network protocol type (tcp,udp,unix)'"`
	Certificate *cert.ClientConfig `yaml:"certificate,omitempty" unic:"certificate,omitempty,desc='Connection encryption settings'"`
}
