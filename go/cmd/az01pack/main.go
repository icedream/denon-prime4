// Command az01pack rebuilds an AZ01 firmware image from an existing one,
// optionally swapping out the stored data of individual partitions. Hashes are
// recomputed by the builder, so a modified partition needs no manual hashing.
//
//	az01pack -base stock.img -replace rootfs=rootfs.img.xz -o patched.img
//
// Replacement data must already be stored in the compression recorded for that
// partition in the base image ("none" or "xz"), as az01pack does not compress.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/icedream/denon-prime4/go/pkg/az01"
)

type replacementList struct {
	byPartition map[string]string
	order       []string
}

func (r *replacementList) String() string { return strings.Join(r.order, ",") }

func (r *replacementList) Set(value string) error {
	name, path, ok := strings.Cut(value, "=")
	if !ok || name == "" || path == "" {
		return fmt.Errorf("expected <partition>=<file>, got %q", value)
	}
	if _, seen := r.byPartition[name]; !seen {
		r.order = append(r.order, name)
	}
	if r.byPartition == nil {
		r.byPartition = map[string]string{}
	}
	r.byPartition[name] = path
	return nil
}

func main() {
	base := flag.String("base", "", "existing image to copy the header and partitions from")
	out := flag.String("o", "", "image to write")
	var replacements replacementList
	flag.Var(&replacements, "replace", "store this file instead of a partition's data (repeatable)")
	flag.Parse()

	if *base == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}

	baseFile, err := os.Open(*base)
	if err != nil {
		log.Fatalf("open base image: %v", err)
	}
	defer baseFile.Close()

	info, err := baseFile.Stat()
	if err != nil {
		log.Fatalf("stat base image: %v", err)
	}

	image, err := az01.Parse(baseFile, info.Size())
	if err != nil {
		log.Fatalf("parse base image: %v", err)
	}

	buildPartitions := make([]az01.BuildPartition, 0, len(image.Partitions))
	for _, partition := range image.Partitions {
		build := az01.BuildPartition{
			Name:        partition.Name,
			Compression: partition.Compression,
			HashAlgo:    partition.HashAlgo,
			Flags:       partition.Flags,
			Data:        image.SectionReader(&partition),
			DataSize:    int64(partition.DataSize),
		}

		if path, ok := replacements.byPartition[partition.Name]; ok {
			replacement, err := os.Open(path)
			if err != nil {
				log.Fatalf("open replacement for %s: %v", partition.Name, err)
			}
			defer replacement.Close()

			stat, err := replacement.Stat()
			if err != nil {
				log.Fatalf("stat replacement for %s: %v", partition.Name, err)
			}
			build.Data = replacement
			build.DataSize = stat.Size()
			log.Printf("partition %s: %d -> %d bytes (%s)", partition.Name,
				partition.DataSize, stat.Size(), partition.Compression)
		}

		buildPartitions = append(buildPartitions, build)
	}

	for _, name := range replacements.order {
		found := false
		for _, partition := range image.Partitions {
			if partition.Name == name {
				found = true
				break
			}
		}
		if !found {
			log.Fatalf("replacement requested for unknown partition %q (image has %d partitions)",
				name, len(image.Partitions))
		}
	}

	outFile, err := os.Create(*out)
	if err != nil {
		log.Fatalf("create output image: %v", err)
	}
	defer outFile.Close()

	builder := az01.Builder{Header: image.Header, Partitions: buildPartitions}
	written, err := builder.WriteTo(outFile)
	if err != nil {
		log.Fatalf("write image: %v", err)
	}

	log.Printf("wrote %s (%d bytes, build %s)", *out, written, image.Header.BuildIdentifier)
}
