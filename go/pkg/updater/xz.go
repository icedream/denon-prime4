package updater

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// footerLen defines the length of the footer.
const footerLen = 12

// Minimum and maximum for the size of the index (backward size).
const (
	minIndexSize = 4
	maxIndexSize = (1 << 32) * 4
)

var ErrFooterMagicMismatch = errors.New("footer magic mismatch")

func getXZUncompressedLength(r io.ReadSeeker) (int64, error) {
	// read footer and after all safety checks extract backward size from it
	if _, err := r.Seek(-footerLen, io.SeekEnd); err != nil {
		return 0, fmt.Errorf("failed to seek to footer: %w", err)
	}
	footerBytes := make([]byte, footerLen)
	if n, err := r.Read(footerBytes); err != nil {
		return 0, fmt.Errorf("failed to read footer bytes: %w", err)
	} else if n != footerLen {
		return 0, fmt.Errorf("failed to read footer bytes: %w", ErrInvalidLength)
	}
	if string(footerBytes[10:12]) != "YZ" {
		return 0, fmt.Errorf("failed to read footer bytes: %w", ErrFooterMagicMismatch)
	}
	checksum := binary.LittleEndian.Uint32(footerBytes[0:4])
	calculatedChecksum := crc32.ChecksumIEEE(footerBytes[4:10])
	if checksum != calculatedChecksum {
		return 0, fmt.Errorf("failed to read footer bytes: %w", ErrChecksumMismatch)
	}
	backwardSize := int64(binary.LittleEndian.Uint32(footerBytes[4:8])+1) * 4
	// streamFlags := footerBytes[8:10]

	// get xz index offset from backwardsize and seek to it
	if _, err := r.Seek(-(backwardSize + footerLen), io.SeekEnd); err != nil {
		return 0, fmt.Errorf("failed to seek to index: %w", err)
	}
	br := lzma.ByteReader(r)

	// verify this is actually the index using the index marker
	indexMarker, err := br.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("failed to read index marker: %w", err)
	}
	if indexMarker != 0 {
		return 0, fmt.Errorf("invalid index marker")
	}

	// parse number of records
	numberOfRecords, _, err := readUvarint(br)
	if err != nil {
		return 0, fmt.Errorf("failed to read number of records from index: %w", err)
	}
	if numberOfRecords < 0 {
		return 0, fmt.Errorf("failed to read number of records from index: %w", errors.New("number of records negative"))
	}

	// calculate total uncompressed size from all records
	var totalUncompressedRecordSize int64
	for i := uint64(0); i < numberOfRecords; i++ {
		// skip unpadded size
		_, _, err := readUvarint(br)
		if err != nil {
			return 0, fmt.Errorf("failed to read index record %d: %w", i, err)
		}

		// read uncompressed size for this record and add it
		uncompressedRecordSize, _, err := readUvarint(br)
		if err != nil {
			return 0, fmt.Errorf("failed to read uncompressed size for index record %d: %w", i, err)
		}
		if uncompressedRecordSize < 0 {
			return 0, fmt.Errorf("failed to read uncompressed size for index record %d: %w", i, errors.New("uncompressed size negative"))
		}
		totalUncompressedRecordSize += int64(uncompressedRecordSize)
	}
	return totalUncompressedRecordSize, nil
}
