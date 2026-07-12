package az01

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// eofSentinel is the 16-byte terminator written after the last partition's
// data. Its first 4 bytes are the ASCII "EOF\x00" magic, followed by a
// little-endian uint32 that has so far always been observed as 16 (0x10),
// and 8 bytes of zero padding. The exact meaning of the 0x10 value has not
// been identified.
//
// Some images (e.g. those shared by the Denon DJ PRIME 4/PRIME 2/Numark
// Mixstream Pro) have 4 extra zero bytes *before* this sentinel; others
// (e.g. Denon DJ PRIME GO/SC5000/SC5000M/SC6000/SC6000M) do not. [Parse]
// accepts both forms, but [Builder] always writes the shorter, 16-byte
// form, since the extra leading zeroes do not appear to carry meaning.
var eofSentinel = func() []byte {
	b := make([]byte, 16)
	copy(b[0:4], []byte(eofMagic))
	binary.LittleEndian.PutUint32(b[4:8], 0x10)
	return b
}()

// BuildPartition describes one partition to be written by
// [Builder.WriteTo].
type BuildPartition struct {
	// Name is the partition name, e.g. "splash", "recoverysplash" or
	// "rootfs".
	Name string
	// Compression names the compression already applied to Data, i.e.
	// "none" or "xz". When "xz", Data must already contain a complete,
	// valid .xz stream (with footer) - Builder does not compress data
	// itself.
	Compression string
	// HashAlgo names the hash algorithm to compute over Data and record in
	// the partition entry. Currently only "sha1" is supported, matching
	// every AZ01/SC6000 image observed so far.
	HashAlgo string
	// Flags is written verbatim into the partition entry. Use 1 if unsure;
	// it is the only value observed in the wild.
	Flags uint32
	// Data provides the partition's (possibly compressed) content. It is
	// read twice: once to compute its hash, once to copy it to the output,
	// which is why an io.ReaderAt is required rather than a plain
	// io.Reader.
	Data io.ReaderAt
	// DataSize is the exact number of bytes to read from Data.
	DataSize int64
}

// Builder assembles a new AZ01/SC6000 image from a [Header] and a set of
// [BuildPartition]s, e.g. reconstructed from an existing image after
// modifying and recompressing its root filesystem (see [Image] for reading
// an existing image apart into these pieces).
//
// Header.HeaderSize is computed automatically from the encoded header and
// must be left at its zero value (or match exactly what would be computed,
// which is rarely useful to pre-compute by hand).
type Builder struct {
	Header     Header
	Partitions []BuildPartition
}

// WriteTo encodes the full image - header, every partition's metadata and
// data, and the terminating EOF sentinel - to w, computing each
// partition's content hash as its data streams through. It returns the
// total number of bytes written, matching the io.WriterTo interface.
func (b Builder) WriteTo(w io.Writer) (int64, error) {
	var headerBuf bytes.Buffer
	if err := b.Header.encode(&headerBuf); err != nil {
		return 0, fmt.Errorf("az01: encoding header: %w", err)
	}

	var total int64
	n, err := w.Write(headerBuf.Bytes())
	total += int64(n)
	if err != nil {
		return total, err
	}

	for _, bp := range b.Partitions {
		h, err := newHasher(bp.HashAlgo)
		if err != nil {
			return total, fmt.Errorf("az01: partition %q: %w", bp.Name, err)
		}
		if _, err := io.Copy(h, io.NewSectionReader(bp.Data, 0, bp.DataSize)); err != nil {
			return total, fmt.Errorf("az01: hashing partition %q: %w", bp.Name, err)
		}
		hashValue := h.Sum(nil)

		var entryBuf bytes.Buffer
		if err := encodePartitionEntry(&entryBuf, bp.Name, uint64(bp.DataSize), bp.Compression, bp.Flags, bp.HashAlgo, hashValue); err != nil {
			return total, fmt.Errorf("az01: encoding partition entry %q: %w", bp.Name, err)
		}
		n, err = w.Write(entryBuf.Bytes())
		total += int64(n)
		if err != nil {
			return total, err
		}

		copied, err := io.Copy(w, io.NewSectionReader(bp.Data, 0, bp.DataSize))
		total += copied
		if err != nil {
			return total, fmt.Errorf("az01: writing partition %q data: %w", bp.Name, err)
		}
	}

	n, err = w.Write(eofSentinel)
	total += int64(n)
	if err != nil {
		return total, err
	}

	return total, nil
}
