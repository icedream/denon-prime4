//go:build libxz
// +build libxz

package xzutil

import (
	"io"

	"github.com/jamespfennell/xz"
)

// NewReader returns a streaming decompressor for the .xz data read from r.
//
// This build uses a cgo binding to liblzma, which is generally much faster
// than the pure-Go decoder used by default (see reader_go.go). Enable with
// `go build -tags libxz`.
func NewReader(r io.Reader) (io.ReadCloser, error) {
	dr := xz.NewReader(r)
	return io.NopCloser(dr), nil
}
