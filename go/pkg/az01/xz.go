package az01

import (
	"io"

	"github.com/icedream/denon-prime4/go/pkg/xzutil"
)

// newXZReader wraps r in a streaming .xz decompressor.
func newXZReader(r io.Reader) (io.ReadCloser, error) {
	return xzutil.NewReader(r)
}

// xzUncompressedLength returns the uncompressed size of the .xz stream
// readable via r, by parsing its footer/index rather than decompressing it.
func xzUncompressedLength(r io.ReadSeeker) (int64, error) {
	return xzutil.UncompressedLength(r)
}
