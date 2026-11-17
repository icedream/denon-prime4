package az01

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ulikunitz/xz"
)

// writeU32 appends a little-endian uint32 to buf, for hand-crafting test
// fixtures that don't go through [Builder].
func writeU32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}

// xzWriterForTest returns a streaming .xz compressor writing to w, used to
// build small self-contained test fixtures without depending on an
// external xz binary.
func xzWriterForTest(w io.Writer) (io.WriteCloser, error) {
	return xz.NewWriter(w)
}

// findRepoRoot walks up from the current working directory looking for a
// ".git" directory, returning its parent, or "" if none was found (e.g. in
// a build environment where the .git directory has been stripped).
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
