/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import "go.osspkg.com/network/listen"

type (
	Config struct {
		Address string `yaml:"address"`
		Network string `yaml:"network"`
		SSL     *SSL   `yaml:"ssl,omitempty"`
	}
	SSL struct {
		Certs      []listen.Certificate `yaml:"certs,omitempty"`
		NextProtos []string             `yaml:"next_protos,omitempty"`
	}
)
