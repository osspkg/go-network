/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.ru>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package udpframe

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestNewID(t *testing.T) {
	for range 100 {
		id, err := NewID()
		if err != nil {
			t.Fatal(err)
		}
		if id == 0 {
			t.Fatal("NewID() returned zero")
		}
	}
}

func TestFragmentCount(t *testing.T) {
	for _, tc := range []struct {
		size int
		want uint32
	}{{0, 1}, {1, 1}, {FragmentSize, 1}, {FragmentSize + 1, 2}, {MaxMessageSize, uint32((MaxMessageSize + FragmentSize - 1) / FragmentSize)}} {
		if got := FragmentCount(tc.size); got != tc.want {
			t.Errorf("FragmentCount(%d) = %d, want %d", tc.size, got, tc.want)
		}
	}
}

func TestWriteMessageParseAndReassemble(t *testing.T) {
	data := bytes.Repeat([]byte("udp-frame"), 500)
	var writer packetWriter
	if err := WriteMessage(&writer, 1234, data); err != nil {
		t.Fatal(err)
	}
	if want := int(FragmentCount(len(data))); len(writer.packets) != want {
		t.Fatalf("wrote %d fragments, want %d", len(writer.packets), want)
	}

	var reassembler Reassembler
	var got []byte
	for i := len(writer.packets) - 1; i >= 0; i-- {
		fragment, err := Parse(writer.packets[i])
		if err != nil {
			t.Fatalf("Parse(fragment %d): %v", i, err)
		}
		if fragment.ID != 1234 || fragment.Index != uint32(i) || fragment.Total != uint32(len(data)) {
			t.Fatalf("unexpected fragment metadata: %+v", fragment)
		}
		message, complete, err := reassembler.Add(fragment)
		if err != nil {
			t.Fatalf("Add(fragment %d): %v", i, err)
		}
		if complete {
			got = message
		}
	}
	if !bytes.Equal(got, data) {
		t.Fatal("reassembled message differs from original")
	}
}

func TestWriteMessageFromAndValidation(t *testing.T) {
	data := []byte("streamed payload")
	var writer packetWriter
	if err := WriteMessageFrom(&writer, 9, len(data), bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	fragment, err := Parse(writer.packets[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fragment.Payload, data) {
		t.Fatalf("payload = %q, want %q", fragment.Payload, data)
	}

	for _, tc := range []struct {
		name string
		id   uint64
		size int
		r    io.Reader
	}{
		{name: "zero ID", id: 0, size: 0, r: bytes.NewReader(nil)},
		{name: "negative size", id: 1, size: -1, r: bytes.NewReader(nil)},
		{name: "oversized", id: 1, size: MaxMessageSize + 1, r: bytes.NewReader(nil)},
		{name: "short source", id: 1, size: 2, r: bytes.NewReader([]byte{1})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out packetWriter
			if err := WriteMessageFrom(&out, tc.id, tc.size, tc.r); err == nil {
				t.Fatal("expected error")
			}
		})
	}

	short := packetWriter{short: true}
	if err := WriteMessage(&short, 1, []byte{1}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short writer error = %v, want io.ErrShortWrite", err)
	}
	failing := packetWriter{err: errors.New("write failed")}
	if err := WriteMessage(&failing, 1, []byte{1}); err == nil {
		t.Fatal("expected writer error")
	}
}

func TestParseRejectsInvalidFragments(t *testing.T) {
	valid := encodeTestFragment(1, 0, 1, 1, []byte{1})
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "short header", data: valid[:HeaderSize-1]},
		{name: "wrong magic", data: append([]byte("BADMAGIC"), valid[8:]...)},
		{name: "oversized datagram", data: append(append([]byte(nil), valid...), make([]byte, DatagramSize)...)},
		{name: "zero id", data: encodeTestFragment(0, 0, 1, 1, []byte{1})},
		{name: "too large total", data: encodeTestFragment(1, 0, 1, MaxMessageSize+1, []byte{1})},
		{name: "wrong count", data: encodeTestFragment(1, 0, 2, 1, []byte{1})},
		{name: "index out of range", data: encodeTestFragment(1, 1, 1, 1, []byte{1})},
		{name: "wrong payload length", data: encodeTestFragment(1, 0, 1, 2, []byte{1})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse(tc.data); err == nil {
				t.Fatal("Parse() succeeded for invalid fragment")
			}
		})
	}
}

func TestReassemblerIgnoresOtherIDsAndDuplicates(t *testing.T) {
	first := Fragment{ID: 7, Index: 0, Count: 2, Total: uint32(FragmentSize + 1), Payload: bytes.Repeat([]byte{'a'}, FragmentSize)}
	last := Fragment{ID: 7, Index: 1, Count: 2, Total: uint32(FragmentSize + 1), Payload: []byte{'b'}}
	var r Reassembler
	if _, complete, err := r.Add(first); err != nil || complete {
		t.Fatalf("first Add = complete %v, error %v", complete, err)
	}
	if _, complete, err := r.Add(Fragment{ID: 8}); err != nil || complete {
		t.Fatalf("other-ID Add = complete %v, error %v", complete, err)
	}
	if _, complete, err := r.Add(first); err != nil || complete {
		t.Fatalf("duplicate Add = complete %v, error %v", complete, err)
	}
	message, complete, err := r.Add(last)
	if err != nil || !complete {
		t.Fatalf("last Add = complete %v, error %v", complete, err)
	}
	if len(message) != FragmentSize+1 || message[len(message)-1] != 'b' {
		t.Fatal("reassembled output has unexpected content")
	}
}

func TestDatagramConnRoundTripAndReset(t *testing.T) {
	peer := &responsePeer{responses: make(chan []byte, 1)}
	conn := NewDatagramConn(peer)
	conn.BeginCall()
	request := []byte("hello")
	if n, err := conn.Write(request); err != nil || n != len(request) {
		t.Fatalf("Write() = (%d, %v)", n, err)
	}
	if !conn.RequestStarted() {
		t.Fatal("RequestStarted() = false after Write")
	}
	response := make([]byte, 5)
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != "world" {
		t.Fatalf("response = %q, want world", response)
	}
	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("Read after response = %v, want EOF", err)
	}
	conn.BeginCall()
	if conn.RequestStarted() {
		t.Fatal("BeginCall did not reset request state")
	}
}

func TestDatagramConnEmptyResponse(t *testing.T) {
	peer := &responsePeer{responses: make(chan []byte, 1), empty: true}
	conn := NewDatagramConn(peer)
	conn.BeginCall()
	if _, err := conn.Write([]byte("request")); err != nil {
		t.Fatal(err)
	}
	if n, err := conn.Read(make([]byte, 1)); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("Read() = (%d, %v), want (0, EOF)", n, err)
	}
}

type packetWriter struct {
	packets [][]byte
	short   bool
	err     error
}

func (w *packetWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.short {
		return len(p) - 1, nil
	}
	w.packets = append(w.packets, append([]byte(nil), p...))
	return len(p), nil
}

func encodeTestFragment(id uint64, index, count, total uint32, payload []byte) []byte {
	buf := make([]byte, HeaderSize+len(payload))
	copy(buf, magic)
	binary.BigEndian.PutUint64(buf[8:16], id)
	binary.BigEndian.PutUint32(buf[16:20], index)
	binary.BigEndian.PutUint32(buf[20:24], count)
	binary.BigEndian.PutUint32(buf[24:28], total)
	copy(buf[HeaderSize:], payload)
	return buf
}

type responsePeer struct {
	responses chan []byte
	empty     bool
	mu        sync.Mutex
}

func (p *responsePeer) Read(b []byte) (int, error) {
	packet := <-p.responses
	return copy(b, packet), nil
}

func (p *responsePeer) Write(b []byte) (int, error) {
	request, err := Parse(b)
	if err != nil {
		return 0, err
	}
	response := []byte("world")
	if p.empty {
		response = nil
	}
	var framed packetWriter
	if err := WriteMessage(&framed, request.ID, response); err != nil {
		return 0, err
	}
	p.responses <- framed.packets[0]
	return len(b), nil
}

func (*responsePeer) Close() error                     { return nil }
func (*responsePeer) LocalAddr() net.Addr              { return testAddr("local") }
func (*responsePeer) RemoteAddr() net.Addr             { return testAddr("remote") }
func (*responsePeer) SetDeadline(time.Time) error      { return nil }
func (*responsePeer) SetReadDeadline(time.Time) error  { return nil }
func (*responsePeer) SetWriteDeadline(time.Time) error { return nil }

type testAddr string

func (a testAddr) Network() string { return "udp" }
func (a testAddr) String() string  { return string(a) }

var _ net.Conn = (*responsePeer)(nil)
