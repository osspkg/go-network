/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package internal

import "fmt"

var passableNetwork = map[string]struct{}{
	"tcp":  {},
	"udp":  {},
	"unix": {},
}

func IsPassableNetwork(network string) error {
	if _, ok := passableNetwork[network]; !ok {
		return fmt.Errorf("invalid network type, use: tcp, udp, unix")
	}
	return nil
}
