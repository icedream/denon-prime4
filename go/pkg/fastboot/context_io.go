package fastboot

import "context"

type ContextReader interface {
	ReadContext(context.Context, []byte) (int, error)
}

type ContextWriter interface {
	WriteContext(context.Context, []byte) (int, error)
}
