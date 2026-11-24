/*
 *  Copyright (c) 2024-2026 Mikhail Knyazhev <markus621@yandex.com>. All rights reserved.
 *  Use of this source code is governed by a BSD 3-Clause license that can be found in the LICENSE file.
 */

package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"go.osspkg.com/errors"

	"go.osspkg.com/network/proto/udpframe"
)

const (
	udpMaxInFlight    = 64
	udpMaxPendingData = 32 << 20
	udpMemoryUnit     = 64 << 10
	udpMessageTimeout = 30 * time.Second
)

type udpRequest struct {
	addr        net.Addr
	id          uint64
	data        []byte
	budgetUnits int
}

type udpMessageKey struct {
	peer string
	id   uint64
}

type udpMessageState struct {
	updated      time.Time
	reservedSize int
	assembler    udpframe.Reassembler
}

func (v *object) serveDatagrams(ctx context.Context, conn net.PacketConn) error {
	jobs := make(chan udpRequest, v.maxConnections)
	completedBudget := make(chan struct{}, udpMaxPendingData/udpMemoryUnit)
	responseBudget := make(chan struct{}, udpMaxPendingData/udpMemoryUnit)
	for range v.maxConnections {
		v.workers.Add(1)
		go func() {
			defer v.workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case request, ok := <-jobs:
					if !ok {
						return
					}
					v.handleDatagram(ctx, conn, request, responseBudget)
					releaseBudget(completedBudget, request.budgetUnits)
				}
			}
		}()
	}
	defer close(jobs)

	buffer := make([]byte, udpframe.DatagramSize+1)
	states := make(map[udpMessageKey]*udpMessageState)
	var pendingBytes int
	for {
		if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			return errors.Wrapf(err, "set UDP read deadline")
		}
		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			if ctx.Err() != nil || v.isClosed() {
				return nil
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				cutoff := time.Now().Add(-udpMessageTimeout)
				for key, state := range states {
					if state.updated.Before(cutoff) {
						pendingBytes -= state.reservedSize
						delete(states, key)
					}
				}
				continue
			}
			return errors.Wrapf(err, "read UDP fragment")
		}
		fragment, err := udpframe.Parse(buffer[:n])
		if err != nil {
			continue
		}
		key := udpMessageKey{peer: addr.Network() + "\x00" + addr.String(), id: fragment.ID}
		state := states[key]
		if state == nil {
			reservedSize := int(fragment.Total)
			if len(states) >= udpMaxInFlight || pendingBytes+reservedSize > udpMaxPendingData {
				continue
			}
			state = &udpMessageState{reservedSize: reservedSize}
			states[key] = state
			pendingBytes += reservedSize
		}
		message, complete, err := state.assembler.Add(fragment)
		if err != nil {
			pendingBytes -= state.reservedSize
			delete(states, key)
			continue
		}
		state.updated = time.Now()
		if !complete {
			continue
		}
		pendingBytes -= state.reservedSize
		delete(states, key)
		budgetUnits := (len(message) + udpMemoryUnit - 1) / udpMemoryUnit
		if !reserveBudget(completedBudget, budgetUnits) {
			continue
		}
		request := udpRequest{addr: addr, id: fragment.ID, data: message, budgetUnits: budgetUnits}
		select {
		case jobs <- request:
		case <-ctx.Done():
			releaseBudget(completedBudget, budgetUnits)
			return nil
		}
	}
}

func reserveBudget(budget chan struct{}, units int) bool {
	reserved := 0
	for ; reserved < units; reserved++ {
		select {
		case budget <- struct{}{}:
		default:
			releaseBudget(budget, reserved)
			return false
		}
	}
	return true
}

func releaseBudget(budget chan struct{}, units int) {
	for range units {
		<-budget
	}
}

func (v *object) handleDatagram(ctx context.Context, conn net.PacketConn, request udpRequest, responseBudget chan struct{}) {
	rw := &udpMessageRW{
		reader:   bytes.NewReader(request.data),
		response: udpResponse{budget: responseBudget},
	}
	defer func() { releaseBudget(responseBudget, rw.response.units) }()
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("network server: UDP handler panic from %s: %v", request.addr, recovered)
		}
	}()
	if err := v.handler(ctx, rw, request.addr); err != nil {
		if ctx.Err() == nil {
			log.Printf("network server: UDP handler from %s failed: %v", request.addr, err)
		}
		return
	}
	if rw.response.failure != nil {
		if ctx.Err() == nil {
			log.Printf("network server: UDP response to %s failed: %v", request.addr, rw.response.failure)
		}
		return
	}
	if err := udpframe.WriteMessageFrom(udpAddrWriter{conn: conn, addr: request.addr}, request.id, rw.response.size, &rw.response); err != nil && ctx.Err() == nil {
		log.Printf("network server: UDP response to %s failed: %v", request.addr, err)
	}
}

type udpMessageRW struct {
	reader   *bytes.Reader
	response udpResponse
}

func (v *udpMessageRW) Read(p []byte) (int, error)  { return v.reader.Read(p) }
func (v *udpMessageRW) Write(p []byte) (int, error) { return v.response.Write(p) }

type udpResponse struct {
	blocks     [][]byte
	size       int
	readOffset int
	units      int
	budget     chan struct{}
	failure    error
}

func (v *udpResponse) Write(p []byte) (int, error) {
	remaining := udpframe.MaxMessageSize - v.size
	tooLarge := len(p) > remaining
	if tooLarge {
		p = p[:remaining]
	}

	written := 0
	for len(p) > 0 {
		blockIndex := v.size / udpMemoryUnit
		blockOffset := v.size % udpMemoryUnit
		if blockIndex == len(v.blocks) {
			if !reserveBudget(v.budget, 1) {
				v.failure = errors.New("UDP response memory limit exceeded")
				return written, v.failure
			}
			v.blocks = append(v.blocks, make([]byte, udpMemoryUnit))
			v.units++
		}
		n := copy(v.blocks[blockIndex][blockOffset:], p)
		v.size += n
		written += n
		p = p[n:]
	}
	if tooLarge {
		v.failure = errors.Wrap(errors.New(fmt.Sprintf("UDP response exceeds maximum message size of %d bytes", udpframe.MaxMessageSize)))
		return written, v.failure
	}
	return written, nil
}

func (v *udpResponse) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if v.readOffset >= v.size {
		return 0, io.EOF
	}
	n := 0
	for len(p) > 0 && v.readOffset < v.size {
		blockIndex := v.readOffset / udpMemoryUnit
		blockOffset := v.readOffset % udpMemoryUnit
		available := min(udpMemoryUnit-blockOffset, v.size-v.readOffset)
		copied := copy(p, v.blocks[blockIndex][blockOffset:blockOffset+available])
		p = p[copied:]
		n += copied
		v.readOffset += copied
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

type udpAddrWriter struct {
	conn net.PacketConn
	addr net.Addr
}

func (v udpAddrWriter) Write(p []byte) (int, error) { return v.conn.WriteTo(p, v.addr) }
