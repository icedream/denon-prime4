package az01

import (
	"crypto/sha1"
	"fmt"
	"hash"
)

// newHasher returns a fresh [hash.Hash] for the given AZ01 hash algorithm
// name (as found in [Partition.HashAlgo]).
//
// Only "sha1" is currently supported; it is the only algorithm observed in
// any AZ01/SC6000 image so far (the AZ0x format is documented to use
// SHA-256, but this package does not parse AZ0x images yet - see
// [ErrUnsupportedFormat]).
func newHasher(algo string) (hash.Hash, error) {
	switch algo {
	case "sha1":
		return sha1.New(), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedHashAlgorithm, algo)
	}
}

// hashAlgoID returns the raw numeric identifier AZ01 images use for a given
// hash algorithm name. Only "sha1" (id 4) has been observed in the wild.
func hashAlgoID(algo string) (uint32, error) {
	switch algo {
	case "sha1":
		return 4, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrUnsupportedHashAlgorithm, algo)
	}
}

// compressionID returns the raw numeric identifier AZ01 images use for a
// given compression algorithm name.
func compressionID(name string) (uint32, error) {
	switch name {
	case "none":
		return 4, nil
	case "xz":
		return 2, nil
	default:
		return 0, fmt.Errorf("az01: unsupported compression %q", name)
	}
}
