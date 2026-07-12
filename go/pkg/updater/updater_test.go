package updater

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
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

// TestLoadFirmwareImage_AZ01 is an integration test verifying that
// loadFirmwareImage correctly detects and parses real AZ01-format Engine OS
// 5.x update images (gitignored, not checked in due to size; skipped
// entirely if none are present locally).
func TestLoadFirmwareImage_AZ01(t *testing.T) {
	root := findRepoRoot(t)
	if root == "" {
		t.Skip("could not locate repository root")
	}

	candidates := []string{
		"PRIME4-5.0.0-Update.img",
		"PRIME2-5.0.0-Update.img",
		"MIXSTREAMPRO-5.0.0-Update.img",
		"PRIMEGO-5.0.0-Update.img",
		"SC5000-5.0.0-Update.img",
		"SC5000M-5.0.0-Update.img",
		"SC6000-5.0.0-Update.img",
		"SC6000M-5.0.0-Update.img",
	}

	tested := 0
	for _, name := range candidates {
		path := filepath.Join(root, name)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		f.Close()

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
// present locally).
func TestLoadFirmwareImage_FIT(t *testing.T) {
	root := findRepoRoot(t)
	if root == "" {
		t.Skip("could not locate repository root")
	}

	candidates := []string{
		"PRIME4-4.3.4-Update.img.dtb",
		"PRIME4-4.3.3-Update.img.dtb",
		"PRIMEGO-4.3.4-Update.img.dtb",
		"PRIMEGO-4.3.3-Update.img.dtb",
	}

	tested := 0
	for _, name := range candidates {
		path := filepath.Join(root, name)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		f.Close()

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
