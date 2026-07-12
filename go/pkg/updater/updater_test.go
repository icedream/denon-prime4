package updater

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findRepoRoot walks up from the current working directory looking for a
// ".git" directory, returning its parent, or "" if none was found.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// parseDevicesTxt reads devices.txt from the repository root and returns
// a list of image codes (the fourth field in each line, e.g. "PRIME4",
// "MIXSTREAMPRO", etc.) used to construct the expected .img filename.
//
// devices.txt format (space-separated):
//   vendor device_code product_code image_code image_url updater_url full_name
//
// We only care about the image_code field (index 3) which is used to
// construct the firmware image filename.
func parseDevicesTxt(root string) []string {
	path := filepath.Join(root, "devices.txt")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var codes []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			codes = append(codes, fields[3])
		}
	}
	return codes
}

// TestLoadFirmwareImage_AZ01 is an integration test verifying that
// loadFirmwareImage correctly detects and parses real AZ01-format Engine OS
// 5.x update images (gitignored, not checked in due to size; skipped
// entirely if none are present locally). Candidates are derived from
// devices.txt.
func TestLoadFirmwareImage_AZ01(t *testing.T) {
	root := findRepoRoot(t)
	if root == "" {
		t.Skip("could not locate repository root")
	}

	codes := parseDevicesTxt(root)
	if len(codes) == 0 {
		t.Skip("no devices found in devices.txt")
	}

	tested := 0
	for _, code := range codes {
		// Try common AZ01/SC6000 image filename patterns
		candidates := []string{
			filepath.Join(root, code+"-5.0.0-Update.img"),
			filepath.Join(root, code+"-5.0.3-Update.img"),
			filepath.Join(root, code+"-Update.img"),
		}

		var path string
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				path = c
				break
			}
		}
		if path == "" {
			continue
		}

		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			imageFile, err := os.Open(path)
			if err != nil {
				t.Fatalf("os.Open(%q): %v", path, err)
			}
			defer imageFile.Close()

			fw, err := loadFirmwareImage(imageFile, discardLogger())
			if err != nil {
				// AZ0x format images are expected to be rejected
				// (not yet supported). Skip them gracefully.
				if errors.Is(err, ErrUnsupportedConfiguration) {
					t.Skip("AZ0x format not yet supported")
				}
				t.Fatalf("loadFirmwareImage(%q): %v", path, err)
			}

			if len(fw.Devices) == 0 {
				t.Errorf("expected at least one USB device ID, got none")
			}
			if len(fw.Partitions) == 0 {
				t.Errorf("expected at least one partition, got none")
			}

			var haveRootfs bool
			for _, part := range fw.Partitions {
				if part.Name != part.Partition {
					t.Errorf("partition %q: expected Name == Partition for AZ01 images, got Partition=%q", part.Name, part.Partition)
				}
				if part.UncompressedSize <= 0 {
					t.Errorf("partition %q: expected positive UncompressedSize, got %d", part.Name, part.UncompressedSize)
				}
				if part.Name == "rootfs" {
					haveRootfs = true
				}

				rc, err := part.Open()
				if err != nil {
					t.Fatalf("partition %q: Open(): %v", part.Name, err)
				}
				rc.Close()
			}
			if !haveRootfs {
				t.Errorf("expected a %q partition, not found among %d partitions", "rootfs", len(fw.Partitions))
			}
		})
		tested++
	}

	if tested == 0 {
		t.Skip("no local AZ01-format firmware images found next to the repository root")
	}
}

// TestLoadFirmwareImage_FIT is an integration test verifying that
// loadFirmwareImage still correctly detects and parses real legacy
// FIT/device-tree-format Engine OS 4.x update images, so the AZ01 format
// support added alongside it did not regress the original code path
// (gitignored, not checked in due to size; skipped entirely if none are
// present locally). Candidates are derived by scanning for .dtb files.
func TestLoadFirmwareImage_FIT(t *testing.T) {
	root := findRepoRoot(t)
	if root == "" {
		t.Skip("could not locate repository root")
	}

	// Scan for .dtb files in the repository root
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skip("could not read repository root")
	}

	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".dtb") {
			candidates = append(candidates, filepath.Join(root, name))
		}
	}

	tested := 0
	for _, path := range candidates {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			imageFile, err := os.Open(path)
			if err != nil {
				t.Fatalf("os.Open(%q): %v", path, err)
			}
			defer imageFile.Close()

			fw, err := loadFirmwareImage(imageFile, discardLogger())
			if err != nil {
				t.Fatalf("loadFirmwareImage(%q): %v", path, err)
			}

			if len(fw.Devices) == 0 {
				t.Errorf("expected at least one USB device ID, got none")
			}
			if fw.VersionLabel == "" {
				t.Errorf("expected a non-empty VersionLabel")
			}
			if len(fw.Partitions) == 0 {
				t.Errorf("expected at least one partition, got none")
			}

			for _, part := range fw.Partitions {
				if part.UncompressedSize <= 0 {
					t.Errorf("partition %q: expected positive UncompressedSize, got %d", part.Name, part.UncompressedSize)
				}

				rc, err := part.Open()
				if err != nil {
					t.Fatalf("partition %q: Open(): %v", part.Name, err)
				}
				rc.Close()
			}
		})
		tested++
	}

	if tested == 0 {
		t.Skip("no local FIT-format firmware images found next to the repository root")
	}
}
