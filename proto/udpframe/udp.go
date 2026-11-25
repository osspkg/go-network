/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

// Package udpframe implements the bounded UDP message framing shared by client
// and server, as well as the client's stream-shaped connection adapter.
package udpframe

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"go.osspkg.com/errors"
)

const (
	HeaderSize     = 28
	DatagramSize   = 1200
	FragmentSize   = DatagramSize - HeaderSize
	MaxMessageSize = 16 << 20
	magic          = "GONETUD1"
)

var (
	framePool = sync.Pool{New: func() any { return new([DatagramSize]byte) }}
	readPool  = sync.Pool{New: func() any { buffer := make([]byte, DatagramSize+1); return &buffer }}
)

type Fragment struct {
	ID      uint64
	Index   uint32
	Count   uint32
	Total   uint32
	Payload []byte
}

// DatagramConn presents one framed UDP request and response as an io.ReadWriter.
type DatagramConn struct {
	net.Conn
	request        []byte
	requestStarted bool
	requestSent    bool
	writeErr       error
	responseID     uint64
	response       []byte
	responseLoaded bool
	responseRead   bool
}

// NewDatagramConn wraps a connected UDP socket with buffered message semantics.
func NewDatagramConn(conn net.Conn) *DatagramConn {
	return &DatagramConn{Conn: conn}
}

// BeginCall resets the adapter state before starting another pooled request.
func (v *DatagramConn) BeginCall() {
	v.request = nil
	v.requestStarted = false
	v.requestSent = false
	v.writeErr = nil
	v.responseID = 0
	v.response = nil
	v.responseLoaded = false
	v.responseRead = false
}

// RequestStarted reports whether the current handler wrote a request.
func (v *DatagramConn) RequestStarted() bool { return v.requestStarted }

// FlushRequest sends the buffered request fragments. Read calls this lazily.
func (v *DatagramConn) FlushRequest() error {
	if v.requestSent {
		return nil
	}
	if v.writeErr != nil {
		return v.writeErr
	}
	id, err := NewID()
	if err != nil {
		return err
	}
	v.responseID = id
	if err := WriteMessage(v.Conn, v.responseID, v.request); err != nil {
		return err
	}
	v.requestStarted = true
	v.requestSent = true
	v.request = nil
	return nil
}

func (v *DatagramConn) Write(p []byte) (int, error) {
	if v.requestSent {
		if !v.responseRead {
			return 0, errors.New("previous UDP response has not been fully read")
		}
		v.BeginCall()
	}

	v.requestStarted = true
	remaining := MaxMessageSize - len(v.request)
	if len(p) > remaining {
		v.request = append(v.request, p[:remaining]...)
		v.writeErr = fmt.Errorf("UDP request exceeds maximum message size of %d bytes", MaxMessageSize)
		return remaining, v.writeErr
	}
	v.request = append(v.request, p...)
	return len(p), nil
}

func (v *DatagramConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if v.responseRead {
		return 0, io.EOF
	}
	if !v.responseLoaded {
		if err := v.FlushRequest(); err != nil {
			return 0, err
		}
		response, err := readMessage(v.Conn, v.responseID)
		if err != nil {
			return 0, err
		}
		v.response = response
		v.responseLoaded = true
		if len(response) == 0 {
			v.responseRead = true
			return 0, io.EOF
		}
	}

	n := copy(p, v.response)
	v.response = v.response[n:]
	if len(v.response) == 0 {
		v.response = nil
		v.responseRead = true
	}
	return n, nil
}

// NewID generates a non-zero identifier for a request/response pair.
func NewID() (uint64, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return 0, errors.Wrapf(err, "generate UDP message ID")
	}
	id := binary.BigEndian.Uint64(value[:])
	if id == 0 {
		id = 1
	}
	return id, nil
}

// Parse validates a single GONETUD1 datagram and returns its fragment metadata.
func Parse(data []byte) (Fragment, error) {
	if len(data) < HeaderSize || string(data[:len(magic)]) != magic {
		return Fragment{}, errors.New("invalid UDP fragment header")
	}
	if len(data) > DatagramSize {
		return Fragment{}, errors.New("UDP fragment exceeds datagram size limit")
	}

	fragment := Fragment{
		ID:      binary.BigEndian.Uint64(data[8:16]),
		Index:   binary.BigEndian.Uint32(data[16:20]),
		Count:   binary.BigEndian.Uint32(data[20:24]),
		Total:   binary.BigEndian.Uint32(data[24:28]),
		Payload: data[HeaderSize:],
	}
	if fragment.ID == 0 || fragment.Total > MaxMessageSize {
		return Fragment{}, errors.New("invalid UDP fragment metadata")
	}
	expectedCount := FragmentCount(int(fragment.Total))
	if fragment.Count != expectedCount || fragment.Index >= fragment.Count {
		return Fragment{}, errors.New("invalid UDP fragment index or count")
	}
	start := int(fragment.Index) * FragmentSize
	expectedSize := min(FragmentSize, int(fragment.Total)-start)
	if fragment.Total == 0 {
		expectedSize = 0
	}
	if len(fragment.Payload) != expectedSize {
		return Fragment{}, errors.New("invalid UDP fragment payload size")
	}
	return fragment, nil
}

// FragmentCount returns the number of datagrams needed for a message size.
func FragmentCount(size int) uint32 {
	if size == 0 {
		return 1
	}
	return uint32((size + FragmentSize - 1) / FragmentSize)
}

// WriteMessage fragments and writes a message using its request/response ID.
func WriteMessage(w interface{ Write([]byte) (int, error) }, id uint64, data []byte) error {
	return WriteMessageFrom(w, id, len(data), bytes.NewReader(data))
}

// WriteMessageFrom frames exactly size bytes from source and writes them using
// the supplied request/response ID.
func WriteMessageFrom(w interface{ Write([]byte) (int, error) }, id uint64, size int, source io.Reader) error {
	if id == 0 {
		return errors.New("UDP message ID is zero")
	}
	if size < 0 || size > MaxMessageSize {
		return fmt.Errorf("UDP message exceeds maximum size of %d bytes", MaxMessageSize)
	}

	count := FragmentCount(size)
	frame := framePool.Get().(*[DatagramSize]byte)
	defer framePool.Put(frame)
	for index := range count {
		start := int(index) * FragmentSize
		payloadSize := min(FragmentSize, size-start)
		fragment := frame[:HeaderSize+payloadSize]
		copy(fragment, magic)
		binary.BigEndian.PutUint64(fragment[8:16], id)
		binary.BigEndian.PutUint32(fragment[16:20], index)
		binary.BigEndian.PutUint32(fragment[20:24], count)
		binary.BigEndian.PutUint32(fragment[24:28], uint32(size))
		if payloadSize > 0 {
			if _, err := io.ReadFull(source, fragment[HeaderSize:]); err != nil {
				return errors.Wrapf(err, "read UDP message payload")
			}
		}

		n, err := w.Write(fragment)
		if err != nil {
			return err
		}
		if n != len(fragment) {
			return io.ErrShortWrite
		}
	}
	return nil
}

// Reassembler stores only received fragments until the complete message is
// available. Its caller is responsible for bounding the number and lifetime of
// in-flight messages.
type Reassembler struct {
	id            uint64
	total         uint32
	count         uint32
	data          []byte
	received      []uint64
	receivedCount uint32
}

// Add accepts a fragment and reports the complete message when all fragments
// have arrived.
func (r *Reassembler) Add(fragment Fragment) (message []byte, complete bool, err error) {
	if r.received == nil {
		r.id = fragment.ID
		r.total = fragment.Total
		r.count = fragment.Count
		r.data = make([]byte, int(fragment.Total))
		r.received = make([]uint64, (fragment.Count+63)/64)
	} else if fragment.ID != r.id {
		return nil, false, nil
	} else if fragment.Total != r.total || fragment.Count != r.count {
		return nil, false, errors.New("inconsistent UDP fragment metadata")
	}

	word := fragment.Index / 64
	mask := uint64(1) << (fragment.Index % 64)
	if r.received[word]&mask != 0 {
		return nil, false, nil
	}
	start := int(fragment.Index) * FragmentSize
	copy(r.data[start:], fragment.Payload)
	r.received[word] |= mask
	r.receivedCount++
	if r.receivedCount != r.count {
		return nil, false, nil
	}
	return r.data, true, nil
}

func readMessage(conn net.Conn, expectedID uint64) ([]byte, error) {
	bufferRef := readPool.Get().(*[]byte)
	buffer := *bufferRef
	defer readPool.Put(bufferRef)
	var reassembler Reassembler
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			return nil, err
		}
		fragment, err := Parse(buffer[:n])
		if err != nil {
			return nil, err
		}
		if fragment.ID != expectedID {
			continue
		}
		message, complete, err := reassembler.Add(fragment)
		if err != nil {
			return nil, err
		}
		if complete {
			return message, nil
		}
	}
}
