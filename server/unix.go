/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"go.osspkg.com/errors"

	"go.osspkg.com/network/xnet"
)

func cleanStaleUnixSocket(address string) error {
	if isAbstractUnix(address) {
		return nil
	}
	info, err := os.Lstat(address)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "inspect UNIX socket %q", address)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return errors.Wrap(errors.New(fmt.Sprintf("UNIX socket path %q exists and is not a socket", address)))
	}
	conn, dialErr := net.DialTimeout(xnet.NetUNIX, address, 200*time.Millisecond)
	if dialErr == nil {
		_ = conn.Close()
		return errors.Wrap(errors.New(fmt.Sprintf("UNIX socket %q is already in use", address)))
	}
	if !errors.Is(dialErr, syscall.ECONNREFUSED) {
		return errors.Wrapf(dialErr, "check existing UNIX socket %q", address)
	}
	current, err := os.Lstat(address)
	if err != nil || !os.SameFile(info, current) {
		return errors.Wrap(errors.New(fmt.Sprintf("UNIX socket path %q changed while checking it", address)))
	}
	if err := os.Remove(address); err != nil {
		return errors.Wrapf(err, "remove stale UNIX socket %q", address)
	}
	return nil
}

func isAbstractUnix(address string) bool {
	return strings.HasPrefix(address, "@") || strings.HasPrefix(address, "\x00")
}

func (v *object) removeUnixSocket() {
	if v.conf.Network != xnet.NetUNIX || isAbstractUnix(v.conf.Address) || v.unixInfo == nil {
		return
	}
	current, err := os.Lstat(v.conf.Address)
	if err == nil && os.SameFile(v.unixInfo, current) {
		_ = os.Remove(v.conf.Address)
	}
}
