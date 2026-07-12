// Command az01dump inspects, extracts from, verifies, and builds Denon DJ /
// Numark / Akai Engine OS 5.x "AZ01"/"SC6000" firmware update images. It is
// the AZ01-container counterpart to the "dumpimage" tool already used
// elsewhere in this repository for the older, FIT-based 4.x update image
// format.
package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/dustin/go-humanize"
	"github.com/icedream/denon-prime4/go/pkg/az01"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "info":
		err = runInfo(os.Args[2:])
	case "verify":
		err = runVerify(os.Args[2:])
	case "extract":
		err = runExtract(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "az01dump: unknown subcommand %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `az01dump - inspect Denon/Numark/Akai Engine OS 5.x "AZ01" firmware images

Usage:
  az01dump info    <image.img>
  az01dump verify   <image.img>
  az01dump extract <image.img> <partition-name> [output-file]
  az01dump help

Subcommands:
  info      Print the image header and partition table.
  verify    Recompute and check every partition's content hash.
  extract   Write one partition's data to a file (or stdout if omitted).
            Add -d to decompress (e.g. "xz") partitions on the fly.

Note: the variable-length "AZ0x" container format (used by dual-bootloader
devices such as the PRIME 4 Plus, Mixstream Pro Go/Plus and RANE SYSTEM ONE)
is recognized but not yet supported by this tool.
`)
}

func openImage(path string) (*az01.Image, *os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	img, err := az01.Parse(f, st.Size())
	if err != nil {
		f.Close()
		if errors.Is(err, az01.ErrUnsupportedFormat) {
			return nil, nil, fmt.Errorf("%s: %w", path, err)
		}
		return nil, nil, fmt.Errorf("%s: parsing image: %w", path, err)
	}
	return img, f, nil
}

func runInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() != 1 {
		return errors.New("usage: az01dump info <image.img>")
	}

	img, f, err := openImage(fs.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Printf("Format:        %s\n", img.Format)
	fmt.Printf("Version:       %d\n", img.Header.Version)
	fmt.Printf("Header size:   %d bytes (0x%X)\n", img.Header.HeaderSize, img.Header.HeaderSize)
	fmt.Printf("Description:   %s\n", img.Header.Description)
	fmt.Printf("Description 2: %s\n", img.Header.Description2)
	fmt.Printf("Product codes:")
	for _, pc := range img.Header.ProductCodes {
		fmt.Printf(" %s", pc)
	}
	fmt.Println()
	if len(img.Header.USBDeviceIDs) > 0 {
		fmt.Printf("USB VID/PID:   ")
		for i, id := range img.Header.USBDeviceIDs {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s", id)
		}
		fmt.Println()
	}
	fmt.Println()

	fmt.Printf("Partitions:\n")
	for _, p := range img.Partitions {
		fmt.Printf("  %s\n", p.Name)
		fmt.Printf("    offset:      0x%X\n", p.DataOffset())
		fmt.Printf("    stored size: %s (%d bytes)\n", humanize.Bytes(p.DataSize), p.DataSize)
		if p.Compression != "none" {
			if size, err := img.UncompressedSize(p.Name); err == nil {
				fmt.Printf("    real size:   %s (%d bytes)\n", humanize.Bytes(uint64(size)), size)
			}
		}
		fmt.Printf("    compression: %s\n", p.Compression)
		fmt.Printf("    hash:        %s:%s\n", p.HashAlgo, hex.EncodeToString(p.Hash))
	}

	return nil
}

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() != 1 {
		return errors.New("usage: az01dump verify <image.img>")
	}

	img, f, err := openImage(fs.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()

	failed := false
	for _, p := range img.Partitions {
		err := img.VerifyHash(p.Name)
		status := "OK"
		if err != nil {
			status = "FAILED: " + err.Error()
			failed = true
		}
		fmt.Printf("%-20s %s\n", p.Name, status)
	}
	if failed {
		return errors.New("one or more partitions failed hash verification")
	}
	return nil
}

func runExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	decompress := fs.Bool("d", false, "decompress the partition's data on the fly")
	fs.Parse(args)
	if fs.NArg() < 2 || fs.NArg() > 3 {
		return errors.New("usage: az01dump extract [-d] <image.img> <partition-name> [output-file]")
	}

	imagePath := fs.Arg(0)
	partitionName := fs.Arg(1)

	img, f, err := openImage(imagePath)
	if err != nil {
		return err
	}
	defer f.Close()

	p, err := img.Partition(partitionName)
	if err != nil {
		return err
	}

	var src io.Reader
	if *decompress {
		rc, err := img.Open(partitionName)
		if err != nil {
			return err
		}
		defer rc.Close()
		src = rc
	} else {
		src = img.SectionReader(p)
	}

	var out io.Writer = os.Stdout
	if fs.NArg() == 3 {
		outFile, err := os.Create(fs.Arg(2))
		if err != nil {
			return err
		}
		defer outFile.Close()
		out = outFile
	}

	n, err := io.Copy(out, src)
	if err != nil {
		return err
	}
	if fs.NArg() == 3 {
		fmt.Fprintf(os.Stderr, "Wrote %s (%d bytes) to %s\n", humanize.Bytes(uint64(n)), n, fs.Arg(2))
	}
	return nil
}
