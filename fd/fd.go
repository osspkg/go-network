/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package fd

import (
	"net"
	"os"
	"reflect"
	"syscall"
)

// ByConnect returns the descriptor exposed by c, or -1 when it does not expose one.
func ByConnect(c net.Conn) int64 {
	if isNil(c) {
		return -1
	}
	conn, ok := c.(syscall.Conn)
	if !ok {
		return -1
	}
	return BySyscallConn(conn)
}

// BySyscallConn returns the descriptor exposed through syscall.Conn, or -1
// when the connection does not expose one. The descriptor is only valid while
// the connection remains open.
func BySyscallConn(c syscall.Conn) int64 {
	if isNil(c) {
		return -1
	}
	rawConn, err := c.SyscallConn()
	if err != nil {
		return -1
	}
	if rawConn == nil {
		return -1
	}

	var fd int64 = -1
	if err = rawConn.Control(func(rawFD uintptr) {
		fd = int64(rawFD)
	}); err != nil {
		return -1
	}

	return fd
}

// ByFile returns the descriptor owned by f, or -1 when f is nil or closed.
// The descriptor is only valid while the file remains open.
func ByFile(f *os.File) int64 {
	if f == nil {
		return -1
	}
	fd := f.Fd()
	if fd == ^uintptr(0) {
		return -1
	}
	return int64(fd)
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
