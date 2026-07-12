// Package az01 implements a reader (and writer) for the "AZ01" firmware
// update image container format used by Denon DJ / Numark / Akai Engine OS
// 5.x devices such as the Denon DJ PRIME 4 (JC11).
//
// # Format overview
//
// An AZ01 image starts with a variable-length header (magic "AZ01",
// followed by a build description, the list of compatible
// "<vendor>,<product-code>" strings, some still-unidentified per-build
// integers, and a second, human readable description), immediately followed
// by a chain of partition entries. Each partition entry is a small,
// self-describing metadata block (name, compression, content hash)
// immediately followed by that many bytes of the partition's own (possibly
// compressed) data, immediately followed by the next partition entry - and
// so on, until a small "EOF" sentinel is reached.
//
// Earlier reverse-engineering notes in this repository described the
// per-partition marker as one of three distinct 5-byte strings, "PARTP",
// "PARTX" or "PARTL", as if they indicated the partition's position in the
// table (first/middle/last). Precise byte-level analysis (cross-checked
// against eight real firmware images and their embedded content hashes)
// showed this to be a coincidence of formatting: the marker is actually the
// 4-byte ASCII magic "PART" followed by a little-endian uint32 giving the
// byte length of the entry's own metadata header. That length just happens
// to equal a printable ASCII letter (0x50 'P' = 80 bytes, 0x58 'X' = 88
// bytes, 0x4C 'L' = 76 bytes) for the specific partitions observed so far.
//
// Similarly, firmware images were previously thought to have their trailing
// .xz stream (holding the root filesystem) truncated, missing its standard
// 12-byte footer, requiring lenient tools such as binwalk to extract it.
// This turned out to be incorrect: the footer is present and valid, but
// extraction attempts need to stop reading at the exact byte offset given
// by the partition's own DataSize field (see [Partition.DataSize]) instead
// of reading through to the end of the file, which contains unrelated
// trailing container data (namely the "EOF" sentinel, and for some devices
// the beginning of the *next* firmware container if multiple images have
// been concatenated together, e.g. when produced by some repackaging
// tools).
//
// # Related formats
//
// Two related but distinct container formats exist among Engine OS 5.x
// devices:
//
//   - "SC6000": identical to AZ01 (same "AZ01" magic and partition entry
//     encoding), just with a shorter, 136-byte fixed header holding fewer
//     product codes. [Parse] handles this format transparently, since the
//     header's own HeaderSize field is used to locate the partition chain
//     rather than assuming a fixed 160-byte header.
//   - "AZ0x": a substantially different, variable-length container used by
//     dual-bootloader devices (PRIME 4 Plus, Mixstream Pro Go/Plus, RANE
//     SYSTEM ONE) with SHA-256/RSA-2048 signing instead of bare SHA-1
//     hashes. [DetectFormat] recognizes this format by its literal "AZ0x"
//     magic (the 'x' is a literal byte, not a wildcard), but this package
//     does not yet implement parsing it; see [ErrUnsupportedFormat].
package az01

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Format identifies which firmware container format an image uses.
type Format string

const (
	// FormatAZ01 is the primary, fully-supported container format used by
	// most Engine OS 5.x devices (Denon DJ PRIME 4/PRIME 2/PRIME GO/SC5000/
	// SC5000M, Numark Mixstream Pro).
	FormatAZ01 Format = "AZ01"
	// FormatSC6000 is a variant of FormatAZ01 with a shorter fixed header
	// (136 instead of 160 bytes). Used by the Denon DJ SC6000/SC6000M
	// PRIME. Parsing is otherwise identical and fully supported.
	FormatSC6000 Format = "SC6000"
	// FormatAZ0x is the variable-length, SHA-256/RSA-2048-signed container
	// format used by dual-bootloader devices (Denon DJ PRIME 4 Plus, Numark
	// Mixstream Pro Go/Plus, RANE SYSTEM ONE). Not yet implemented; see
	// [ErrUnsupportedFormat].
	FormatAZ0x Format = "AZ0x"
)

const (
	magicAZ01       = "AZ01"
	magicAZ0x       = "AZ0x"
	partitionMagic  = "PART"
	sc6000HeaderLen = 0x88

	// eofMagic is the 4-byte sentinel that terminates the partition chain.
	eofMagic = "EOF\x00"
)

var (
	// ErrNotAnImage is returned when the input does not start with a magic
	// number recognized by this package.
	ErrNotAnImage = errors.New("az01: not a recognized firmware image")
	// ErrUnsupportedFormat is returned by [Parse] when the input is
	// recognized (see [DetectFormat]) but parsing it is not implemented.
	ErrUnsupportedFormat = errors.New("az01: unsupported firmware container format")
	// ErrTruncated is returned when the input ends before an expected field
	// could be fully read.
	ErrTruncated = errors.New("az01: unexpected end of data")
	// ErrUnexpectedByte is returned when a byte does not match what the
	// format's field encoding requires (e.g. a missing NUL terminator or
	// non-zero padding byte). Seeing this error most likely means the
	// image uses a not-yet-understood variation of the format.
	ErrUnexpectedByte = errors.New("az01: unexpected byte in field encoding")
	// ErrBadPartitionMagic is returned when a partition entry was expected
	// but the "PART" magic was not found, and the bytes also did not match
	// the terminating EOF sentinel.
	ErrBadPartitionMagic = errors.New("az01: bad partition entry magic")
	// ErrPartitionEntryLengthMismatch is returned when the number of bytes
	// consumed while parsing a partition entry's metadata does not match
	// the length declared by the entry itself.
	ErrPartitionEntryLengthMismatch = errors.New("az01: partition entry length mismatch")
	// ErrPartitionNotFound is returned by lookups for a partition name that
	// does not exist in the image.
	ErrPartitionNotFound = errors.New("az01: partition not found")
	// ErrHashMismatch is returned by verification functions when the
	// computed content hash does not match the one recorded in the image.
	ErrHashMismatch = errors.New("az01: hash mismatch")
	// ErrUnsupportedHashAlgorithm is returned when a partition entry names a
	// hash algorithm this package does not know how to compute.
	ErrUnsupportedHashAlgorithm = errors.New("az01: unsupported hash algorithm")
)

// DetectFormat sniffs the first 12 bytes available via r and reports which
// firmware container format they belong to. It does not validate the rest
// of the image.
func DetectFormat(r io.ReaderAt) (Format, error) {
	var buf [12]byte
	if _, err := r.ReadAt(buf[:], 0); err != nil && err != io.EOF {
		return "", fmt.Errorf("az01: reading magic: %w", err)
	}
	switch string(buf[0:4]) {
	case magicAZ01:
		headerSize := binary.LittleEndian.Uint32(buf[8:12])
		if headerSize == sc6000HeaderLen {
			return FormatSC6000, nil
		}
		return FormatAZ01, nil
	case magicAZ0x:
		return FormatAZ0x, nil
	default:
		return "", ErrNotAnImage
	}
}

// Header holds the fields of an AZ01/SC6000 image header. Some fields'
// exact purpose has not been conclusively reverse engineered; they are kept
// around verbatim so that [Builder] can round-trip an image without losing
// information.
type Header struct {
	// Version is the header format version. Only 1 has been observed.
	Version uint32
	// HeaderSize is the total size of the header in bytes, and therefore
	// also the absolute file offset of the first partition entry.
	HeaderSize uint32
	// Description is a build identifier, e.g. "SNAPSHOT-20260507060852".
	Description string
	// ProductCodes lists the "<vendor>,<product-code>" strings of devices
	// this image is compatible with, e.g. "inmusic,jc11".
	ProductCodes []string
	// USBDeviceIDs lists the USB vendor/product ID pairs the device is
	// expected to advertise over USB when connected to a host for firmware
	// update. All observed AZ01/SC6000 images use VID 0x15E4 (Numark
	// Industries); the PIDs vary by product family.
	USBDeviceIDs []USBDeviceID
	// Description2 is a second, human-readable description, e.g. "Planck
	// AZ01 Console upgrade Image".
	Description2 string
}

// USBDeviceID represents a USB vendor/product ID pair (VID:PID).
//
// All observed AZ01/SC6000 firmware images use VID 0x15E4 (Numark Industries).
type USBDeviceID struct {
	VendorID  uint16
	ProductID uint16
}

// String returns the USB device ID in "VVVV:PPPP" format.
func (id USBDeviceID) String() string {
	return fmt.Sprintf("%04x:%04x", id.VendorID, id.ProductID)
}

// HeaderSize returns the number of bytes h.Encode would produce, without
// actually encoding it.
func (h Header) EncodedSize() int {
	var buf bytes.Buffer
	// EncodedSize is used before the real HeaderSize is known, so encode
	// into a scratch buffer; headers are tiny (well under 1 KiB) so this is
	// cheap.
	_ = h.encode(&buf)
	return buf.Len()
}

// Partition describes one partition entry: its metadata (as recorded in the
// image) plus, once obtained via [Parse], the location of its data within
// the underlying image.
type Partition struct {
	// Name is the partition's name as used by the bootloader/fastboot
	// protocol on the device, e.g. "splash", "recoverysplash" or "rootfs".
	Name string
	// DataSize is the size, in bytes, of the partition's data as stored in
	// the image - i.e. the *compressed* size when Compression != "none".
	DataSize uint64
	// CompressionID is the raw numeric compression algorithm identifier.
	CompressionID uint32
	// Compression is the compression algorithm name, e.g. "none" or "xz".
	Compression string
	// Flags is a raw bitfield whose meaning has not been identified yet.
	// The value 1 has been observed on every partition entry seen so far.
	Flags uint32
	// HashAlgoID is the raw numeric hash algorithm identifier.
	HashAlgoID uint32
	// HashAlgo is the hash algorithm name, e.g. "sha1". Despite earlier
	// documentation calling this a "signature", it is a bare content hash
	// with no asymmetric cryptography involved - anyone can recompute and
	// replace it after modifying the corresponding data.
	HashAlgo string
	// Hash is the expected hash of the partition's data (as stored, i.e.
	// still compressed if applicable), per HashAlgo.
	Hash []byte

	// dataOffset is the absolute offset of this partition's data within
	// the image. Zero-value Partition structs created outside this package
	// (e.g. for building a new image, see [Builder]) do not need it set.
	dataOffset int64
}

// Image is a fully parsed AZ01/SC6000 firmware image.
type Image struct {
	Format     Format
	Header     Header
	Partitions []Partition

	r io.ReaderAt
	// eofOffset is the absolute offset of the "EOF" sentinel that
	// terminates the partition chain, mostly useful for diagnostics.
	eofOffset int64
	// size is the total size of the underlying image, if known (0 if not).
	size int64
}

// maxHeaderReadSize bounds the initial speculative read used to locate and
// parse the image header. Every header observed so far, across all AZ01 and
// SC6000 devices, is well under 1 KiB; this is deliberately generous.
const maxHeaderReadSize = 1 << 16 // 64 KiB

// maxPartitionEntryReadSize bounds the read used to parse a single
// partition entry's metadata (i.e. excluding the partition's own data).
// Every entry observed so far is under 100 bytes.
const maxPartitionEntryReadSize = 4096

// Parse reads and parses an AZ01 or SC6000 firmware image from r. size, if
// known, should be the total length of the underlying data (e.g. from
// os.File.Stat) and is used only to bound the search for the terminating
// EOF sentinel; pass 0 if unknown.
//
// Parse returns [ErrUnsupportedFormat] (wrapped) for the AZ0x container
// format, which is not implemented by this package.
func Parse(r io.ReaderAt, size int64) (*Image, error) {
	format, err := DetectFormat(r)
	if err != nil {
		return nil, err
	}
	if format == FormatAZ0x {
		return nil, fmt.Errorf("%w: AZ0x container parsing is not implemented (variable-length header, SHA-256/RSA-2048 signing)", ErrUnsupportedFormat)
	}

	headerBuf, err := readAt(r, 0, maxHeaderReadSize)
	if err != nil {
		return nil, fmt.Errorf("az01: reading header: %w", err)
	}
	header, err := parseHeader(headerBuf)
	if err != nil {
		return nil, fmt.Errorf("az01: parsing header: %w", err)
	}

	img := &Image{Format: format, Header: header, r: r, size: size}

	offset := int64(header.HeaderSize)
	for {
		entryBuf, err := readAt(r, offset, maxPartitionEntryReadSize)
		if err != nil {
			return nil, fmt.Errorf("az01: reading partition entry at 0x%X: %w", offset, err)
		}
		if len(entryBuf) >= 4 && string(entryBuf[0:4]) == eofMagic {
			img.eofOffset = offset
			break
		}
		if len(entryBuf) < 4 || string(entryBuf[0:4]) != partitionMagic {
			// Allow for a run of zero padding bytes before the EOF
			// sentinel (observed on some but not all images).
			skip := 0
			for skip < len(entryBuf) && entryBuf[skip] == 0 {
				skip++
			}
			if skip > 0 && skip+4 <= len(entryBuf) && string(entryBuf[skip:skip+4]) == eofMagic {
				img.eofOffset = offset + int64(skip)
				break
			}
			return nil, fmt.Errorf("%w: expected %q or end-of-image marker at offset 0x%X, got %q",
				ErrBadPartitionMagic, partitionMagic, offset, previewBytes(entryBuf))
		}

		part, consumed, err := parsePartitionEntry(entryBuf)
		if err != nil {
			return nil, fmt.Errorf("az01: parsing partition entry at 0x%X: %w", offset, err)
		}
		part.dataOffset = offset + int64(consumed)
		img.Partitions = append(img.Partitions, part)
		offset = part.dataOffset + int64(part.DataSize)
	}

	return img, nil
}

// previewBytes formats up to the first 16 bytes of b as hex, for error
// messages.
func previewBytes(b []byte) string {
	if len(b) > 16 {
		b = b[:16]
	}
	return fmt.Sprintf("%x", b)
}

// readAt reads up to n bytes at offset from r, returning fewer bytes (but no
// error) on a short read that reached the end of the underlying data.
func readAt(r io.ReaderAt, offset int64, n int) ([]byte, error) {
	buf := make([]byte, n)
	read, err := r.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf[:read], nil
}

func parseHeader(data []byte) (Header, error) {
	if len(data) < 12 || string(data[0:4]) != magicAZ01 {
		return Header{}, ErrNotAnImage
	}
	c := newCursor(data, 0)
	c.pos = 4 // skip magic, already checked above

	version, err := c.u32()
	if err != nil {
		return Header{}, err
	}
	headerSize, err := c.u32()
	if err != nil {
		return Header{}, err
	}
	description, err := c.lenPrefixedString()
	if err != nil {
		return Header{}, fmt.Errorf("description: %w", err)
	}
	productCodeCount, err := c.u32()
	if err != nil {
		return Header{}, err
	}
	productCodes := make([]string, productCodeCount)
	for i := range productCodes {
		productCodes[i], err = c.lenPrefixedString()
		if err != nil {
			return Header{}, fmt.Errorf("product code %d: %w", i, err)
		}
	}
	extraCount, err := c.u32()
	if err != nil {
		return Header{}, err
	}
	usbIDs := make([]USBDeviceID, extraCount)
	for i := range usbIDs {
		v, err := c.u32()
		if err != nil {
			return Header{}, fmt.Errorf("USB device ID %d: %w", i, err)
		}
		usbIDs[i] = USBDeviceID{
			VendorID:  uint16(v >> 16),
			ProductID: uint16(v & 0xFFFF),
		}
	}
	description2, err := c.lenPrefixedString()
	if err != nil {
		return Header{}, fmt.Errorf("description2: %w", err)
	}

	if uint32(c.pos) != headerSize {
		return Header{}, fmt.Errorf("az01: header parsing consumed %d bytes but HeaderSize field says %d", c.pos, headerSize)
	}

	return Header{
		Version:      version,
		HeaderSize:   headerSize,
		Description:  description,
		ProductCodes: productCodes,
		USBDeviceIDs: usbIDs,
		Description2: description2,
	}, nil
}

// parsePartitionEntry parses a single partition entry's metadata header
// (i.e. everything up to, but not including, its data) from the start of
// data. It returns the parsed partition (with dataOffset left at zero -
// callers must set it relative to the entry's real file offset) and the
// number of bytes consumed by the metadata header.
func parsePartitionEntry(data []byte) (Partition, int, error) {
	if len(data) < 8 || string(data[0:4]) != partitionMagic {
		return Partition{}, 0, ErrBadPartitionMagic
	}
	entryLen := binary.LittleEndian.Uint32(data[4:8])

	c := newCursor(data, 0)
	c.pos = 8

	dataSize, err := c.u64()
	if err != nil {
		return Partition{}, 0, err
	}
	name, err := c.lenPrefixedString()
	if err != nil {
		return Partition{}, 0, fmt.Errorf("name: %w", err)
	}
	compID, err := c.u32()
	if err != nil {
		return Partition{}, 0, err
	}
	compName, err := c.nulTerminatedString()
	if err != nil {
		return Partition{}, 0, fmt.Errorf("compression name: %w", err)
	}
	flags, err := c.u32()
	if err != nil {
		return Partition{}, 0, err
	}
	hashAlgoID, err := c.u32()
	if err != nil {
		return Partition{}, 0, err
	}
	hashAlgoName, err := c.nulTerminatedString()
	if err != nil {
		return Partition{}, 0, fmt.Errorf("hash algorithm name: %w", err)
	}
	hashLen, err := c.u32()
	if err != nil {
		return Partition{}, 0, err
	}
	hash, err := c.bytesN(int(hashLen))
	if err != nil {
		return Partition{}, 0, fmt.Errorf("hash: %w", err)
	}

	if uint32(c.pos) != entryLen {
		return Partition{}, 0, fmt.Errorf("%w: entry declares %d bytes, parsing consumed %d", ErrPartitionEntryLengthMismatch, entryLen, c.pos)
	}

	return Partition{
		Name:          name,
		DataSize:      dataSize,
		CompressionID: compID,
		Compression:   compName,
		Flags:         flags,
		HashAlgoID:    hashAlgoID,
		HashAlgo:      hashAlgoName,
		Hash:          append([]byte(nil), hash...),
	}, c.pos, nil
}

// Partition looks up a partition by name.
func (img *Image) Partition(name string) (*Partition, error) {
	for i := range img.Partitions {
		if img.Partitions[i].Name == name {
			return &img.Partitions[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrPartitionNotFound, name)
}

// DataOffset returns the absolute offset of a partition's (possibly
// compressed) data within the image, as determined by [Parse].
func (p *Partition) DataOffset() int64 {
	return p.dataOffset
}

// SectionReader returns a reader for exactly the raw (possibly compressed)
// bytes of p's data within img.
func (img *Image) SectionReader(p *Partition) *io.SectionReader {
	return io.NewSectionReader(img.r, p.dataOffset, int64(p.DataSize))
}

// Open returns a reader for the *decompressed* content of the named
// partition. For partitions stored uncompressed ("none"), this is
// equivalent to [Image.SectionReader]. Unknown compression algorithms
// result in an error.
func (img *Image) Open(name string) (io.ReadCloser, error) {
	p, err := img.Partition(name)
	if err != nil {
		return nil, err
	}
	raw := img.SectionReader(p)
	switch p.Compression {
	case "none", "":
		return io.NopCloser(raw), nil
	case "xz":
		return newXZReader(raw)
	default:
		return nil, fmt.Errorf("az01: partition %q uses unsupported compression %q", p.Name, p.Compression)
	}
}

// UncompressedSize returns the decompressed size of the named partition's
// data, without fully decompressing it. For uncompressed partitions this is
// simply DataSize.
func (img *Image) UncompressedSize(name string) (int64, error) {
	p, err := img.Partition(name)
	if err != nil {
		return 0, err
	}
	switch p.Compression {
	case "none", "":
		return int64(p.DataSize), nil
	case "xz":
		return xzUncompressedLength(img.SectionReader(p))
	default:
		return 0, fmt.Errorf("az01: partition %q uses unsupported compression %q", p.Name, p.Compression)
	}
}

// VerifyHash recomputes the hash of the named partition's stored (still
// possibly compressed) data and compares it against the hash recorded in
// the image, returning [ErrHashMismatch] (wrapped) if they differ.
func (img *Image) VerifyHash(name string) error {
	p, err := img.Partition(name)
	if err != nil {
		return err
	}
	h, err := newHasher(p.HashAlgo)
	if err != nil {
		return err
	}
	if _, err := io.Copy(h, img.SectionReader(p)); err != nil {
		return fmt.Errorf("az01: hashing partition %q: %w", name, err)
	}
	actual := h.Sum(nil)
	if !bytes.Equal(actual, p.Hash) {
		return fmt.Errorf("%w: partition %q: expected %x, got %x", ErrHashMismatch, name, p.Hash, actual)
	}
	return nil
}
