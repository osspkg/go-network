package server

import (
	"context"
	"io"
)

type (
	Ctx interface {
		io.Reader
		io.Writer
		Addr() string
		Context() context.Context
	}

	Handler interface {
		Handler(ctx Ctx)
	}
)
