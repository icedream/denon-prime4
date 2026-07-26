package az01

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// cursor is a small helper for decoding the field encoding used throughout
// the AZ01 container format:
//
//   - fixed-size little-endian integers (u32/u64)
//   - length-prefixed strings: a uint32 byte count, followed by that many
//     bytes, followed by a single NUL byte and then zero-padding up to the
//     next 4-byte boundary (relative to the start of the enclosing
//     structure, e.g. the image header or a partition entry)
//   - NUL-terminated strings: bytes up to (and including) a NUL byte,
//     followed by the same zero-padding rule
//
// Every string field observed in this format carries a trailing NUL byte
// even when its length is already known from an explicit length prefix, and
// every field - including plain integers - is subsequently laid out on a
// 4-byte boundary relative to the start of the structure being parsed. The
// cursor's base field tracks that origin.
type cursor struct {
	data []byte
	pos  int
	base int
}

func newCursor(data []byte, base int) *cursor {
	return &cursor{data: data, pos: base, base: base}
}

// off returns the cursor's current position relative to its base.
func (c *cursor) off() int {
	return c.pos - c.base
}

func (c *cursor) require(n int) error {
	if c.pos+n > len(c.data) {
		return fmt.Errorf("%w: need %d bytes at offset %d, have %d", ErrTruncated, n, c.pos, len(c.data)-c.pos)
	}
	return nil
}

func (c *cursor) u32() (uint32, error) {
	if err := c.require(4); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint32(c.data[c.pos : c.pos+4])
	c.pos += 4
	return v, nil
}

func (c *cursor) u64() (uint64, error) {
	if err := c.require(8); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint64(c.data[c.pos : c.pos+8])
	c.pos += 8
	return v, nil
}

func (c *cursor) bytesN(n int) ([]byte, error) {
	if err := c.require(n); err != nil {
		return nil, err
	}
	b := c.data[c.pos : c.pos+n]
	c.pos += n
	return b, nil
}

// alignAfterString consumes the mandatory NUL terminator at the cursor's
// current position and then any further zero bytes up to the next 4-byte
// boundary relative to c.base. It fails if a non-zero byte is encountered
// where padding was expected, since that would indicate the field layout
// assumptions no longer hold for the data being parsed.
func (c *cursor) alignAfterString() error {
	if err := c.require(1); err != nil {
		return err
	}
	if c.data[c.pos] != 0 {
		return fmt.Errorf("%w: expected NUL terminator at offset %d, got 0x%02x", ErrUnexpectedByte, c.pos, c.data[c.pos])
	}
	c.pos++
	for c.off()%4 != 0 {
		if err := c.require(1); err != nil {
			return err
		}
		if c.data[c.pos] != 0 {
			return fmt.Errorf("%w: expected zero padding at offset %d, got 0x%02x", ErrUnexpectedByte, c.pos, c.data[c.pos])
		}
		c.pos++
	}
	return nil
}

// lenPrefixedString reads a uint32 length followed by that many bytes of
// string content, then aligns past the trailing NUL terminator.
func (c *cursor) lenPrefixedString() (string, error) {
	n, err := c.u32()
	if err != nil {
		return "", err
	}
	b, err := c.bytesN(int(n))
	if err != nil {
		return "", err
	}
	s := string(b)
	if err := c.alignAfterString(); err != nil {
		return "", err
	}
	return s, nil
}

// nulTerminatedString reads bytes up to the next NUL byte, then aligns past
// it the same way lenPrefixedString does.
func (c *cursor) nulTerminatedString() (string, error) {
	idx := bytes.IndexByte(c.data[c.pos:], 0)
	if idx < 0 {
		return "", fmt.Errorf("%w: unterminated string starting at offset %d", ErrTruncated, c.pos)
	}
	s := string(c.data[c.pos : c.pos+idx])
	c.pos += idx
	if err := c.alignAfterString(); err != nil {
		return "", err
	}
	return s, nil
}
