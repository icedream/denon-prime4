// Package xzutil provides small helpers around .xz streams that are not
// covered by the third-party decoder libraries already used in this
// repository, namely computing the uncompressed size of a stream without
// fully decompressing it (by parsing the stream's index/footer).
//
// This is a standalone counterpart to the private helpers in
// [github.com/icedream/denon-prime4/go/pkg/updater], duplicated here (rather
// than imported) so that packages such as
// [github.com/icedream/denon-prime4/go/pkg/az01] do not need to depend on
// the USB/fastboot-flashing machinery that the updater package pulls in.
package xzutil

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// FooterLen is the fixed length, in bytes, of an .xz stream footer.
const FooterLen = 12

var (
	// ErrFooterMagicMismatch is returned when the last two bytes of a
	// supposed .xz stream footer are not "YZ".
	ErrFooterMagicMismatch = errors.New("xzutil: footer magic mismatch")
	// ErrFooterChecksumMismatch is returned when the CRC32 embedded in the
	// footer does not match the footer's own contents.
	ErrFooterChecksumMismatch = errors.New("xzutil: footer checksum mismatch")
	// ErrInvalidIndexMarker is returned when the index block was expected
	// but its marker byte was not 0x00.
	ErrInvalidIndexMarker = errors.New("xzutil: invalid index marker")

	errOverflowU64 = errors.New("xzutil: uvarint overflows 64-bit unsigned integer")
)

// UncompressedLength returns the total uncompressed size of the data
// contained in the .xz stream readable via r, by parsing the stream's
// footer and index rather than decompressing the payload.
//
// r must expose a *complete*, standards-compliant .xz stream ending in the
// normal 12-byte footer (magic "YZ", CRC32, backward size, stream flags).
// Firmware images analyzed in this repository were initially thought to
// omit this footer, but subsequent analysis of the "AZ01" container format
// (see [github.com/icedream/denon-prime4/go/pkg/az01]) showed that the
// footer is present as long as the exact partition size recorded in the
// container's partition table is used to bound the stream - earlier
// extraction attempts were simply reading past the end of the real stream
// into unrelated trailing container data.
func UncompressedLength(r io.ReadSeeker) (int64, error) {
	// read footer and after all safety checks extract backward size from it
	if _, err := r.Seek(-FooterLen, io.SeekEnd); err != nil {
		return 0, fmt.Errorf("failed to seek to footer: %w", err)
	}
	footerBytes := make([]byte, FooterLen)
	if n, err := io.ReadFull(r, footerBytes); err != nil {
		return 0, fmt.Errorf("failed to read footer bytes: %w", err)
	} else if n != FooterLen {
		return 0, fmt.Errorf("failed to read footer bytes: short read")
	}
	if string(footerBytes[10:12]) != "YZ" {
		return 0, fmt.Errorf("failed to read footer bytes: %w", ErrFooterMagicMismatch)
	}
	checksum := binary.LittleEndian.Uint32(footerBytes[0:4])
	calculatedChecksum := crc32.ChecksumIEEE(footerBytes[4:10])
	if checksum != calculatedChecksum {
		return 0, fmt.Errorf("failed to read footer bytes: %w", ErrFooterChecksumMismatch)
	}
	backwardSize := int64(binary.LittleEndian.Uint32(footerBytes[4:8])+1) * 4
	// streamFlags := footerBytes[8:10]

	// get xz index offset from backward size and seek to it
	if _, err := r.Seek(-(backwardSize + FooterLen), io.SeekEnd); err != nil {
		return 0, fmt.Errorf("failed to seek to index: %w", err)
	}
	br := lzma.ByteReader(r)

	// verify this is actually the index using the index marker
	indexMarker, err := br.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("failed to read index marker: %w", err)
	}
	if indexMarker != 0 {
		return 0, ErrInvalidIndexMarker
	}

	// parse number of records
	numberOfRecords, _, err := readUvarint(br)
	if err != nil {
		return 0, fmt.Errorf("failed to read number of records from index: %w", err)
	}

	// calculate total uncompressed size from all records
	var totalUncompressedRecordSize int64
	for i := uint64(0); i < numberOfRecords; i++ {
		// skip unpadded size
		if _, _, err := readUvarint(br); err != nil {
			return 0, fmt.Errorf("failed to read index record %d: %w", i, err)
		}

		// read uncompressed size for this record and add it
		uncompressedRecordSize, _, err := readUvarint(br)
		if err != nil {
			return 0, fmt.Errorf("failed to read uncompressed size for index record %d: %w", i, err)
		}
		totalUncompressedRecordSize += int64(uncompressedRecordSize)
	}
	return totalUncompressedRecordSize, nil
}

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
