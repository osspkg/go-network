/*
 *  Copyright (c) 2024-2025 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package address_test

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"go.osspkg.com/casecheck"

	"go.osspkg.com/network/address"
)

func TestUnit_FixIPPort(t *testing.T) {
	tests := []struct {
		name string
		port string
		args []string
		want []string
	}{
		{
			name: "Case1",
			port: "53",
			args: []string{"1.1.1.1", "1.1.1.1:123", "123.11.11"},
			want: []string{"1.1.1.1:53", "1.1.1.1:123"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := address.FixIPPort(tt.port, tt.args...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FixIPPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnit_ResolveIPPort(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{addr: "", want: `127.0.0.1:[0-9]+`},
		{addr: ":", want: `0.0.0.0:[0-9]+`},
		{addr: ":123", want: "0.0.0.0:123"},
		{addr: "1.1.1.1", want: "1.1.1.1:8080"},
		{addr: "1.1.1.1:", want: "1.1.1.1:8080"},
		{addr: "1.1.1.1:123", want: "1.1.1.1:123"},
		{addr: "0.0.0.0:", want: "0.0.0.0:"},
		{addr: "localhost", want: `(127.0.0.1|\[::1\]):[0-9]+`},
		{addr: "localhost:", want: `(127.0.0.1|\[::1\]):[0-9]+`},
		{addr: "localhost:123", want: `(127.0.0.1|\[::1\]):123`},
		{addr: "a.b.c.d:123", want: "a.b.c.d:123"},
		{addr: "::", want: `\[::\]:[0-9]+`},
		{addr: "[::]", want: `\[::\]:[0-9]+`},
		{addr: "[::]:", want: `\[::\]:[0-9]+`},
		{addr: "[::]:123", want: `\[::\]:123`},
		{addr: "/unix.sock", want: "/unix.sock"},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("[Case%d]=>'%s'", i, tt.addr), func(t *testing.T) {
			got := address.ResolveIPPort(tt.addr)

			t.Logf("from `%s` to `%s`", tt.addr, got)

			ok, err := regexp.Match(tt.want, []byte(got))
			casecheck.NoError(t, err, got)
			casecheck.True(t, ok, got)
		})
	}
}
