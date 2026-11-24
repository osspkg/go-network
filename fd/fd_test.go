/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package fd

import (
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
)

func TestByFile(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close() //nolint:errcheck

	if got, want := ByFile(reader), int64(reader.Fd()); got != want || got < 0 {
		t.Fatalf("ByFile() = %d, want descriptor %d", got, want)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if got := ByFile(reader); got != -1 {
		t.Fatalf("ByFile(closed file) = %d, want -1", got)
	}
	if got := ByFile(nil); got != -1 {
		t.Fatalf("ByFile(nil) = %d, want -1", got)
	}
}

func TestByConnectWithoutSyscallDescriptor(t *testing.T) {
	if got := ByConnect(nil); got != -1 {
		t.Fatalf("ByConnect(nil) = %d, want -1", got)
	}
	left, right := net.Pipe()
	defer left.Close()  //nolint:errcheck
	defer right.Close() //nolint:errcheck
	if got := ByConnect(left); got != -1 {
		t.Fatalf("ByConnect(net.Pipe()) = %d, want -1", got)
	}
}

func TestByConnectAndBySyscallConn(t *testing.T) {
	conn := &testConn{raw: &testRawConn{fd: 123}}
	if got := ByConnect(conn); got != 123 {
		t.Fatalf("ByConnect() = %d, want 123", got)
	}
	if got := BySyscallConn(conn); got != 123 {
		t.Fatalf("BySyscallConn() = %d, want 123", got)
	}
	var nilConn *testConn
	if got := ByConnect(nilConn); got != -1 {
		t.Fatalf("ByConnect(typed nil) = %d, want -1", got)
	}
	if got := BySyscallConn(nilConn); got != -1 {
		t.Fatalf("BySyscallConn(typed nil) = %d, want -1", got)
	}
}

func TestBySyscallConnFailures(t *testing.T) {
	syscallErr := errors.New("syscall conn failed")
	controlErr := errors.New("control failed")
	for _, tc := range []struct {
		name string
		conn *testConn
	}{
		{name: "SyscallConn error", conn: &testConn{err: syscallErr}},
		{name: "nil RawConn", conn: &testConn{}},
		{name: "Control error", conn: &testConn{raw: &testRawConn{fd: 123, err: controlErr}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := BySyscallConn(tc.conn); got != -1 {
				t.Fatalf("BySyscallConn() = %d, want -1", got)
			}
		})
	}
}

type testConn struct {
	net.Conn
	raw syscall.RawConn
	err error
}

func (c *testConn) SyscallConn() (syscall.RawConn, error) { return c.raw, c.err }

type testRawConn struct {
	fd  uintptr
	err error
}

func (c *testRawConn) Control(fn func(uintptr)) error {
	if c.err != nil {
		return c.err
	}
	fn(c.fd)
	return nil
}

func (*testRawConn) Read(func(uintptr) bool) error  { return nil }
func (*testRawConn) Write(func(uintptr) bool) error { return nil }
