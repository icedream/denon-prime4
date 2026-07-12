package az01

import (
	"bytes"
	"encoding/binary"
)

// writeCursor mirrors cursor's read-side field encoding rules for writing:
// length-prefixed and NUL-terminated strings are always followed by a NUL
// byte and then zero-padding up to the next 4-byte boundary relative to the
// start of the structure being encoded (a header or a partition entry).
type writeCursor struct {
	buf  *bytes.Buffer
	base int
}

func newWriteCursor(buf *bytes.Buffer) *writeCursor {
	return &writeCursor{buf: buf, base: buf.Len()}
}

func (w *writeCursor) off() int {
	return w.buf.Len() - w.base
}

func (w *writeCursor) u32(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	w.buf.Write(b[:])
}

func (w *writeCursor) u64(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	w.buf.Write(b[:])
}

func (w *writeCursor) alignAfterString() {
	w.buf.WriteByte(0)
	for w.off()%4 != 0 {
		w.buf.WriteByte(0)
	}
}

func (w *writeCursor) lenPrefixedString(s string) {
	w.u32(uint32(len(s)))
	w.buf.WriteString(s)
	w.alignAfterString()
}

func (w *writeCursor) nulTerminatedString(s string) {
	w.buf.WriteString(s)
	w.alignAfterString()
}

// encode writes h's on-disk representation to buf, computing and patching
// in the correct HeaderSize as the final step. Any caller-supplied
// h.HeaderSize value is ignored (encode always derives it from the actual
// encoded length).
func (h Header) encode(buf *bytes.Buffer) error {
	w := newWriteCursor(buf)
	buf.WriteString(magicAZ01)
	w.u32(h.Version)
	headerSizeFieldPos := buf.Len()
	w.u32(0) // placeholder, patched below
	w.lenPrefixedString(h.BuildIdentifier)
	w.u32(uint32(len(h.ProductCodes)))
	for _, pc := range h.ProductCodes {
		w.lenPrefixedString(pc)
	}
	w.u32(uint32(len(h.USBDeviceIDs)))
	for _, id := range h.USBDeviceIDs {
		w.u32(uint32(id.VendorID)<<16 | uint32(id.ProductID))
	}
	w.lenPrefixedString(h.Description)

	total := uint32(w.off())
	binary.LittleEndian.PutUint32(buf.Bytes()[headerSizeFieldPos:headerSizeFieldPos+4], total)
	return nil
}

// encodePartitionEntry writes one partition entry's metadata header (magic,
// entry length, data size, name, compression, flags, hash algorithm and
// hash value) to buf. It does not write the partition's data itself.
func encodePartitionEntry(buf *bytes.Buffer, name string, dataSize uint64, compression string, flags uint32, hashAlgo string, hashValue []byte) error {
	compID, err := compressionID(compression)
	if err != nil {
		return err
	}
	hAlgoID, err := hashAlgoID(hashAlgo)
	if err != nil {
		return err
	}

	w := newWriteCursor(buf)
	buf.WriteString(partitionMagic)
	entryLenFieldPos := buf.Len()
	w.u32(0) // placeholder, patched below
	w.u64(dataSize)
	w.lenPrefixedString(name)
	w.u32(compID)
	w.nulTerminatedString(compression)
	w.u32(flags)
	w.u32(hAlgoID)
	w.nulTerminatedString(hashAlgo)
	w.u32(uint32(len(hashValue)))
	buf.Write(hashValue)

	total := uint32(w.off())
	binary.LittleEndian.PutUint32(buf.Bytes()[entryLenFieldPos:entryLenFieldPos+4], total)
	return nil
}
