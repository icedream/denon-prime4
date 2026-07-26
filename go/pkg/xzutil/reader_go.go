//go:build !libxz
// +build !libxz

package xzutil

import (
	"io"

	"github.com/ulikunitz/xz"
)

// NewReader returns a streaming decompressor for the .xz data read from r.
//
// This build uses the pure-Go decoder from github.com/ulikunitz/xz. Build
// with the "libxz" tag to use a cgo binding to liblzma instead (see
// reader_libxz.go), which is generally much faster.
func NewReader(r io.Reader) (io.ReadCloser, error) {
	xzReader, err := xz.NewReader(r)
	return io.NopCloser(xzReader), err
}
