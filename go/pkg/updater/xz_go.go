//go:build !libxz
// +build !libxz

package updater

import (
	"io"

	"github.com/ulikunitz/xz"
)

func newXZReader(r io.Reader) (io.ReadCloser, error) {
	xzReader, err := xz.NewReader(r)
	return io.NopCloser(xzReader), err
}
