/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import "go.osspkg.com/network/listen"

type (
	Config struct {
		Address string `yaml:"address" unic:"address,default='127.0.0.1:80',desc='ip:port для прослушивания сервером'"`
		Network string `yaml:"network" unic:"network,default='tcp',desc='Тип сетевого протокола (tcp,udp,unix,quic)'"`
		SSL     *SSL   `yaml:"ssl,omitempty" unic:"ssl,omitempty,desc='Настройки шифрования соединения'"`
	}
	SSL struct {
		Certs      []listen.Certificate `yaml:"certs,omitempty" unic:"certs,omitempty,desc='Список сертификатов'"`
		NextProtos []string             `yaml:"next_protos,omitempty" unic:"next_protos,omitempty,desc='Поддерживаемые уровни протоколов'"`
	}
)
