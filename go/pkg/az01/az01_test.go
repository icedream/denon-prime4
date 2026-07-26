package az01

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// deviceEntry represents a single device entry parsed from devices.txt.
type deviceEntry struct {
	ImageCode string
	ImageURL  string
}

// parseDevicesTxt reads devices.txt from the repository root and returns
// a list of device entries. The image code and download URL are extracted
// from each line to derive the expected firmware image filename.
//
// devices.txt format (space-separated):
//   vendor device_code product_code image_code image_url updater_url full_name
func parseDevicesTxt(root string) []deviceEntry {
	path := filepath.Join(root, "devices.txt")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var entries []deviceEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			entries = append(entries, deviceEntry{
				ImageCode: fields[3],
				ImageURL:  fields[4],
			})
		}
	}
	return entries
}

// buildTestImage assembles a small, valid in-memory AZ01 image with three
// partitions (mirroring real-world images: an uncompressed "splash", an
// uncompressed "recoverysplash" and an xz-compressed "rootfs").
func buildTestImage(t *testing.T) []byte {
	t.Helper()

	splash := bytes.Repeat([]byte{0x0a, 0x0a, 0x0a, 0xff}, 16)
	recoverySplash := bytes.Repeat([]byte{0x00, 0x00, 0x00, 0xff}, 16)

	var rootfsXZ bytes.Buffer
	xzw, err := xzWriterForTest(&rootfsXZ)
	require.NoError(t, err)
	_, err = xzw.Write([]byte("hello from a fake rootfs\n"))
	require.NoError(t, err)
	require.NoError(t, xzw.Close())

	b := Builder{
		Header: Header{
			Version:         1,
			BuildIdentifier: "SNAPSHOT-19700101000000",
			ProductCodes:    []string{"inmusic,test1", "inmusic,test2"},
			USBDeviceIDs: []USBDeviceID{
				{VendorID: 0x1111, ProductID: 0x1111},
				{VendorID: 0x2222, ProductID: 0x2222},
			},
			Description: "Test AZ01 image",
		},
		Partitions: []BuildPartition{
			{
				Name:        "splash",
				Compression: "none",
				HashAlgo:    "sha1",
				Flags:       1,
				Data:        bytes.NewReader(splash),
				DataSize:    int64(len(splash)),
			},
			{
				Name:        "recoverysplash",
				Compression: "none",
				HashAlgo:    "sha1",
				Flags:       1,
				Data:        bytes.NewReader(recoverySplash),
				DataSize:    int64(len(recoverySplash)),
			},
			{
				Name:        "rootfs",
				Compression: "xz",
				HashAlgo:    "sha1",
				Flags:       1,
				Data:        bytes.NewReader(rootfsXZ.Bytes()),
				DataSize:    int64(rootfsXZ.Len()),
			},
		},
	}

	var out bytes.Buffer
	_, err = b.WriteTo(&out)
	require.NoError(t, err)
	return out.Bytes()
}

func TestBuildAndParseRoundTrip(t *testing.T) {
	data := buildTestImage(t)
	r := bytes.NewReader(data)

	format, err := DetectFormat(r)
	require.NoError(t, err)
	require.Equal(t, FormatAZ01, format)

	img, err := Parse(r, int64(len(data)))
	require.NoError(t, err)

	require.Equal(t, uint32(1), img.Header.Version)
	require.Equal(t, "SNAPSHOT-19700101000000", img.Header.BuildIdentifier)
	require.Equal(t, []string{"inmusic,test1", "inmusic,test2"}, img.Header.ProductCodes)
	require.Equal(t, []USBDeviceID{
		{VendorID: 0x1111, ProductID: 0x1111},
		{VendorID: 0x2222, ProductID: 0x2222},
	}, img.Header.USBDeviceIDs)
	require.Equal(t, "Test AZ01 image", img.Header.Description)

	// USBDeviceIDs.String() should format correctly
	require.Equal(t, "1111:1111", img.Header.USBDeviceIDs[0].String())
	require.Equal(t, "2222:2222", img.Header.USBDeviceIDs[1].String())

	require.Len(t, img.Partitions, 3)
	names := make([]string, len(img.Partitions))
	for i, p := range img.Partitions {
		names[i] = p.Name
	}
	require.Equal(t, []string{"splash", "recoverysplash", "rootfs"}, names)

	for _, p := range img.Partitions {
		require.NoError(t, img.VerifyHash(p.Name), "partition %q hash should verify", p.Name)
	}

	rc, err := img.Open("rootfs")
	require.NoError(t, err)
	defer rc.Close()
	decompressed, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "hello from a fake rootfs\n", string(decompressed))

	uncompressedSize, err := img.UncompressedSize("rootfs")
	require.NoError(t, err)
	require.Equal(t, int64(len(decompressed)), uncompressedSize)
}

func TestDetectFormat(t *testing.T) {
	t.Run("not an image", func(t *testing.T) {
		_, err := DetectFormat(bytes.NewReader([]byte("not a firmware image at all")))
		require.ErrorIs(t, err, ErrNotAnImage)
	})

	t.Run("SC6000 header size", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteString("AZ01")
		writeU32(&buf, 1)
		writeU32(&buf, sc6000HeaderLen)
		format, err := DetectFormat(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err)
		require.Equal(t, FormatSC6000, format)
	})

	t.Run("AZ0x is recognized but unsupported by Parse", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteString("AZ0x")
		buf.Write(make([]byte, 8))
		r := bytes.NewReader(buf.Bytes())
		format, err := DetectFormat(r)
		require.NoError(t, err)
		require.Equal(t, FormatAZ0x, format)

		_, err = Parse(r, int64(buf.Len()))
		require.ErrorIs(t, err, ErrUnsupportedFormat)
	})
}

// TestParseRealFirmwareImages is an integration test that runs against real
// firmware images if any are found relative to the repository root (they
// are gitignored and not checked in due to their size, but are the actual
// files this package was reverse-engineered against). It is skipped
// entirely if none are present.
func TestParseRealFirmwareImages(t *testing.T) {
	root := findRepoRoot(t)
	if root == "" {
		t.Skip("could not locate repository root")
	}

	entries := parseDevicesTxt(root)
	if len(entries) == 0 {
		t.Skip("no devices found in devices.txt")
	}

	tested := 0
	for _, entry := range entries {
		expectedName := filepath.Base(entry.ImageURL)
		path := filepath.Join(root, expectedName)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		t.Run(expectedName, func(t *testing.T) {
			defer f.Close()
			st, err := f.Stat()
			require.NoError(t, err)

			img, err := Parse(f, st.Size())
			require.NoError(t, err)

			require.NotEmpty(t, img.Partitions)
			for _, p := range img.Partitions {
				require.NoError(t, img.VerifyHash(p.Name), "partition %q hash should verify", p.Name)
			}

			// The root filesystem partition should decompress to a
			// substantial ext2/ext4 filesystem image; just sanity-check its
			// declared uncompressed size rather than fully decompressing
			// ~500 MiB in a unit test.
			if _, err := img.Partition("rootfs"); err == nil {
				size, err := img.UncompressedSize("rootfs")
				require.NoError(t, err)
				require.Greater(t, size, int64(100<<20), "rootfs should decompress to well over 100 MiB")
			}

			// USB VID/PID pairs should be present and use VID 0x15E4 (Numark)
			require.NotEmpty(t, img.Header.USBDeviceIDs, "AZ01/SC6000 images should have USB VID/PID pairs")
			for _, id := range img.Header.USBDeviceIDs {
				require.Equal(t, uint16(0x15E4), id.VendorID, "all AZ01/SC6000 USB VIDs should be 0x15E4 (Numark)")
				require.NotZero(t, id.ProductID, "USB PIDs should be non-zero")
			}
		})
		tested++
	}
	if tested == 0 {
		t.Skip("no real firmware images found next to the repository root; skipping integration test")
	}
}

// TestRepackRoundTrip verifies that parsing a real AZ01/SC6000 firmware image
// and then rebuilding it with [Builder] produces a bit-exact reconstruction
// of the original (modulo the EOF sentinel, which the Builder writes in a
// canonical 16-byte form regardless of what the original contained).
func TestRepackRoundTrip(t *testing.T) {
	root := findRepoRoot(t)
	if root == "" {
		t.Skip("could not locate repository root")
	}

	entries := parseDevicesTxt(root)
	if len(entries) == 0 {
		t.Skip("no devices found in devices.txt")
	}

	tested := 0
	for _, entry := range entries {
		expectedName := filepath.Base(entry.ImageURL)
		path := filepath.Join(root, expectedName)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		t.Run(expectedName, func(t *testing.T) {
			defer f.Close()
			st, err := f.Stat()
			require.NoError(t, err)

			// Skip AZ0x format images (not supported).
			format, err := DetectFormat(f)
			require.NoError(t, err)
			if format == FormatAZ0x {
				t.Skip("AZ0x format not supported")
			}

			// Read the original image bytes.
			original, err := io.ReadAll(io.LimitReader(f, st.Size()))
			require.NoError(t, err)

			// Parse the image.
			img, err := Parse(f, st.Size())
			require.NoError(t, err)

			// Extract partition data (raw, compressed if applicable).
			var buildPartitions []BuildPartition
			for i := range img.Partitions {
				p := &img.Partitions[i]
				data, err := readPartitionRawData(img, p)
				require.NoError(t, err, "partition %q data should be readable", p.Name)
				buildPartitions = append(buildPartitions, BuildPartition{
					Name:        p.Name,
					Compression: p.Compression,
					HashAlgo:    p.HashAlgo,
					Flags:       p.Flags,
					Data:        bytes.NewReader(data),
					DataSize:    int64(len(data)),
				})
			}

			// Rebuild the image.
			builder := Builder{
				Header:     img.Header,
				Partitions: buildPartitions,
			}
			var rebuilt bytes.Buffer
			_, err = builder.WriteTo(&rebuilt)
			require.NoError(t, err, "builder should produce valid output")

			// Compare original with rebuilt, ignoring the EOF sentinel
			// which may differ in length (16 vs 20 bytes).
			originalWithoutEOF := stripEOFSentinel(original)
			rebuiltWithoutEOF := stripEOFSentinel(rebuilt.Bytes())
			require.Equal(t, originalWithoutEOF, rebuiltWithoutEOF,
				"repack should produce bit-exact output (modulo EOF sentinel)")
		})
		tested++
	}
	if tested == 0 {
		t.Skip("no real firmware images found next to the repository root; skipping integration test")
	}
}

// readPartitionRawData reads the raw (possibly compressed) data of a
// partition from an [Image], without decompressing it.
func readPartitionRawData(img *Image, p *Partition) ([]byte, error) {
	raw := img.SectionReader(p)
	return io.ReadAll(io.LimitReader(raw, int64(p.DataSize)))
}

// stripEOFSentinel removes the trailing EOF sentinel from data, returning
// only the header + partition entries + partition data. This is necessary
// because the Builder writes a canonical 16-byte EOF sentinel, while some
// original images may have a 20-byte sentinel (with 4 extra leading zeros).
func stripEOFSentinel(data []byte) []byte {
	// Find the EOF sentinel. It starts with "EOF\x00".
	idx := bytes.Index(data, []byte("EOF\x00"))
	if idx < 0 {
		// No sentinel found; return the whole thing.
		return data
	}
	// Strip any leading zero bytes before the sentinel (some images have
	// 4 extra zero bytes before the EOF magic).
	for idx > 0 && data[idx-1] == 0 {
		idx--
	}
	return data[:idx]
}
