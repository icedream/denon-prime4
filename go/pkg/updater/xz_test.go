package updater

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed assets/test/lorem_ipsum.txt.xz
var compressedAsset []byte

//go:embed assets/test/lorem_ipsum.txt
var uncompressedAsset []byte

func TestGetXZUncompressedLength(t *testing.T) {
	uncompressedLength, err := getXZUncompressedLength(bytes.NewReader(compressedAsset))
	require.NoError(t, err)
	require.Equal(t, int64(len(uncompressedAsset)), uncompressedLength)
}
