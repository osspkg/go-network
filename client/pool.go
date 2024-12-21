package client

import (
	"io"
	"time"

	"go.osspkg.com/network/internal"
)

type action interface {
	io.Reader
	io.Writer
	internal.TDeadline
}

type connect struct {
	Conn      action
	CloseFunc func()
	Err       error
	IdleAt    time.Time
	Timeout   time.Duration
}

func (c *connect) Close() {
	if c.CloseFunc != nil {
		c.CloseFunc()
	}
}

func (c *connect) IsFailConn() bool {
	return c.Err != nil || time.Now().Add(c.Timeout).After(c.IdleAt)
}

func (c *connect) GetError() error {
	return c.Err
}

type (
	object interface {
		Close()
		IsFailConn() bool
		GetError() error
	}

	chanPool[T object] struct {
		c    chan T
		call func() T
	}
)

func newChanPool[T object](size int, call func() T) *chanPool[T] {
	return &chanPool[T]{
		c:    make(chan T, size+1),
		call: call,
	}
}

func (p *chanPool[T]) GetIdleOrCreateConn() (v T) {
	var isNew bool
	for {
		select {
		case v = <-p.c:
		default:
			v = p.call()
			isNew = true
		}

		if isNew {
			return
		}

		if v.IsFailConn() {
			v.Close()
			continue
		}

		return
	}
}

func (p *chanPool[T]) PutOrCloseIdleConn(v T) {
	if v.IsFailConn() {
		v.Close()
		return
	}

	select {
	case p.c <- v:
		return
	default:
		v.Close()
	}
}
