/*
 *  Copyright (c) 2024 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package fd

import (
	"net"
	"reflect"
)

func ByConnect(c net.Conn) int64 {
	fd := reflect.Indirect(reflect.ValueOf(c)).FieldByName("fd")
	pfd := reflect.Indirect(fd).FieldByName("pfd")
	return pfd.FieldByName("Sysfd").Int()
}
