//go:build libxz
// +build libxz

package updater

import (
	"io"

	"github.com/jamespfennell/xz"
)

func newXZReader(r io.Reader) (io.ReadCloser, error) {
	dr := xz.NewReader(r)
	return io.NopCloser(dr), nil
}
