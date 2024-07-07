package updater

import (
	"errors"
	"io"
)

// errOverflow indicates an overflow of the 64-bit unsigned integer.
//
// Adapted from https://github.com/ulikunitz/xz/blob/master/bits.go#L53.
var errOverflowU64 = errors.New("uvarint overflows 64-bit unsigned integer")

// readUvarint reads a uvarint from the given byte reader.
//
// Adapted from https://github.com/ulikunitz/xz/blob/master/bits.go#L56.
func readUvarint(r io.ByteReader) (x uint64, n int, err error) {
	const maxUvarintLen = 10

	var s uint
	i := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return x, i, err
		}
		i++
		if i > maxUvarintLen {
			return x, i, errOverflowU64
		}
		if b < 0x80 {
			if i == maxUvarintLen && b > 1 {
				return x, i, errOverflowU64
			}
			return x | uint64(b)<<s, i, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
}
