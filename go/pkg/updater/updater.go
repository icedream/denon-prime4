package updater

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/gousb"
	"github.com/icedream/denon-prime4/go/pkg/az01"
	"github.com/icedream/denon-prime4/go/pkg/fastboot"
	"github.com/sqweek/dialog"
	"github.com/u-root/u-root/pkg/dt"
)

var (
	ErrNoMatchingDevices        = errors.New("no matching devices")
	ErrNoImagesInDeviceTree     = errors.New("no images in device tree")
	ErrMissingVersion           = errors.New("missing version")
	ErrBadVersion               = errors.New("bad version")
	ErrUnsupportedConfiguration = errors.New("unsupported configuration")
	ErrChecksumMismatch         = errors.New("checksum mismatch")
)

// TODO - Max packet size must be 64 bytes for full-speed, 512 bytes for high-speed and 1024 bytes for Super Speed USB.

type Progress struct {
	Text         string
	Percentage   float64
	Indetermined bool
	Cancellable  bool
}

var ErrInvalidLength = errors.New("invalid length")

type DeviceID struct {
	VendorID, ProductID uint16
}

func (id DeviceID) String() string {
	return fmt.Sprintf("%04x:%04x", id.VendorID, id.ProductID)
}

func bytesAsDeviceList(b []byte) ([]DeviceID, error) {
	if len(b)%4 != 0 {
		return nil, ErrInvalidLength
	}
	items := make([]DeviceID, len(b)/4)
	i := 0
	for offset := 0; offset < len(b); offset += 4 {
		items[i].VendorID = binary.BigEndian.Uint16(b[offset : offset+2])
		items[i].ProductID = binary.BigEndian.Uint16(b[offset+2 : offset+4])
		i++
	}
	return items, nil
}

// firmwareImage is a format-agnostic view of a parsed update image. It
// abstracts over the two container formats currently supported:
//
//   - the legacy FIT/flattened-device-tree format used by Engine OS 4.x
//     update images (parsed by [parseFITImage])
//   - the AZ01/SC6000 container format used by Engine OS 5.x update images
//     (parsed by [parseAZ01Image], backed by [github.com/icedream/denon-prime4/go/pkg/az01])
//
// so that the rest of the updater (device communication, download/flash
// loop, progress reporting) does not need to know which format is in use.
type firmwareImage struct {
	// Devices lists the USB VID/PID pairs of devices this image is meant
	// to be flashed to.
	Devices []DeviceID
	// VersionLabel is a human-readable identifier of the firmware build,
	// used only for progress text. For FIT images this is the
	// "inmusic,version" property (a semantic version such as "4.3.4").
	// For AZ01/SC6000 images this is the header's build description
	// (e.g. "SNAPSHOT-20260507060852"), since that container format does
	// not carry a separate semantic version field.
	VersionLabel string
	// Partitions lists, in the order they must be written, every
	// partition contained in the image.
	Partitions []firmwareImagePartition
}

// firmwareImagePartition is one partition to be downloaded and flashed as
// part of a firmware update, already hash-verified by the time it is
// returned from [loadFirmwareImage].
type firmwareImagePartition struct {
	// Name identifies the partition for logging/progress purposes.
	Name string
	// Partition is the fastboot partition name to flash the data to.
	Partition string
	// UncompressedSize is the decompressed size of this partition's data,
	// used both to size the fastboot "download" call and for progress
	// reporting.
	UncompressedSize int64
	// Open returns a fresh reader over this partition's decompressed
	// data. It is called once per partition during the download phase.
	Open func() (io.ReadCloser, error)
}

// loadFirmwareImage detects which container format imageFile holds and
// parses it into a format-agnostic [firmwareImage], verifying every
// partition's content hash in the process.
func loadFirmwareImage(imageFile *os.File, logger *slog.Logger) (*firmwareImage, error) {
	format, err := az01.DetectFormat(imageFile)
	switch {
	case err == nil:
		if format == az01.FormatAZ0x {
			return nil, fmt.Errorf("%w: AZ0x firmware images are not supported yet", ErrUnsupportedConfiguration)
		}
		logger.Debug("Detected firmware image format", "format", format)
		return parseAZ01Image(imageFile, logger)
	case errors.Is(err, az01.ErrNotAnImage):
		logger.Debug("Image does not match a known AZ01/SC6000/AZ0x magic, assuming legacy FIT format")
		return parseFITImage(imageFile, logger)
	default:
		return nil, fmt.Errorf("detecting firmware image format: %w", err)
	}
}

// parseFITImage parses the legacy FIT/flattened-device-tree update image
// format used by Engine OS 4.x.
func parseFITImage(imageFile *os.File, logger *slog.Logger) (*firmwareImage, error) {
	fdt, err := dt.New(dt.WithReaderAt(imageFile))
	if err != nil {
		return nil, err
	}

	// extract list of compatible devices
	devices := fdt.Root().Property("inmusic,devices")
	devicesBytes, err := devices.AsBytes()
	if err != nil {
		return nil, err
	}
	devicesList, err := bytesAsDeviceList(devicesBytes)
	if err != nil {
		return nil, err
	}

	// extract version string
	version := fdt.Root().Property("inmusic,version")
	if version == nil {
		return nil, ErrMissingVersion
	}
	versionStr, err := version.AsString()
	if err != nil {
		return nil, ErrBadVersion
	}

	images := fdt.Root().Walk("images")
	if images == nil {
		return nil, ErrNoImagesInDeviceTree
	}
	imageNames, err := images.ListChildNodes()
	if err != nil {
		return nil, err
	}

	fw := &firmwareImage{
		Devices:      devicesList,
		VersionLabel: versionStr,
	}

	for _, imageName := range imageNames {
		image := images.Walk(imageName)
		if image == nil {
			continue
		}

		partition := image.Property("partition")
		if partition == nil {
			return nil, errors.New("missing partition")
		}
		partitionStr, err := partition.AsString()
		if err != nil {
			return nil, err
		}

		data := image.Property("data")
		if data == nil {
			return nil, errors.New("missing data")
		}
		dataBytes, err := data.AsBytes()
		if err != nil {
			return nil, err
		}

		compressionStr := ""
		if compression := image.Property("compression"); compression != nil {
			compressionStr, err = compression.AsString()
			if err != nil {
				return nil, err
			}
		}

		var uncompressedSize int64
		switch compressionStr {
		case "", "none":
			uncompressedSize = int64(len(dataBytes))
		case "xz":
			uncompressedSize, err = getXZUncompressedLength(bytes.NewReader(dataBytes))
			if err != nil {
				return nil, err
			}
		default:
			logger.Error("Unsupported compression",
				"compression", compressionStr,
				"imageName", imageName)
			return nil, errors.New("unsupported compression: " + compressionStr)
		}

		// verify image hash (computed over the stored, possibly still
		// compressed bytes, matching AZ01 semantics)
		hashProp := image.Walk("hash")
		if hashProp == nil {
			return nil, errors.New("missing image hash")
		}
		hashAlgo := hashProp.Property("algo")
		if hashAlgo == nil {
			return nil, errors.New("missing image hash algorithm")
		}
		hashAlgoStr, err := hashAlgo.AsString()
		if err != nil {
			return nil, err
		}
		hashValue := hashProp.Property("value")
		if hashValue == nil {
			return nil, errors.New("missing image hash value")
		}
		hashBytes, err := hashValue.AsBytes()
		if err != nil {
			return nil, err
		}
		var hasher hash.Hash
		switch hashAlgoStr {
		case "sha1":
			hasher = sha1.New()
		default:
			logger.Error("Checksum algorithm not supported",
				"imageName", imageName,
				"hashAlgo", hashAlgoStr)
			return nil, errors.New("checksum algorithm not supported yet: " + hashAlgoStr)
		}
		logger.Info("Verifying image checksum",
			"imageName", imageName,
			"hashAlgo", hashAlgoStr,
			"wantedHash", hex.EncodeToString(hashBytes))
		if _, err := hasher.Write(dataBytes); err != nil {
			logger.Error("Failed to generate checksum",
				"imageName", imageName,
				"err", err)
			return nil, fmt.Errorf("checksum generation failure: %w", err)
		}
		actualHash := hasher.Sum(nil)
		if !bytes.Equal(actualHash, hashBytes) {
			logger.Error("Checksum mismatch",
				"imageName", imageName,
				"hashAlgo", hashAlgoStr,
				"wantedHash", hex.EncodeToString(hashBytes),
				"actualHash", hex.EncodeToString(actualHash))
			return nil, ErrChecksumMismatch
		}
		logger.Info("Image checksum OK",
			"imageName", imageName)

		imageNameCopy := imageName
		compressionStrCopy := compressionStr
		dataBytesCopy := dataBytes
		fw.Partitions = append(fw.Partitions, firmwareImagePartition{
			Name:             imageNameCopy,
			Partition:        partitionStr,
			UncompressedSize: uncompressedSize,
			Open: func() (io.ReadCloser, error) {
				var r io.Reader = bytes.NewReader(dataBytesCopy)
				switch compressionStrCopy {
				case "", "none":
					return io.NopCloser(r), nil
				case "xz":
					return newXZReader(r)
				default:
					return nil, errors.New("unsupported compression: " + compressionStrCopy)
				}
			},
		})
	}

	return fw, nil
}

// parseAZ01Image parses the AZ01/SC6000 update image container format used
// by Engine OS 5.x.
func parseAZ01Image(imageFile *os.File, logger *slog.Logger) (*firmwareImage, error) {
	info, err := imageFile.Stat()
	if err != nil {
		return nil, err
	}

	img, err := az01.Parse(imageFile, info.Size())
	if err != nil {
		return nil, err
	}

	devicesList := make([]DeviceID, len(img.Header.USBDeviceIDs))
	for i, id := range img.Header.USBDeviceIDs {
		devicesList[i] = DeviceID{VendorID: id.VendorID, ProductID: id.ProductID}
	}

	fw := &firmwareImage{
		Devices:      devicesList,
		VersionLabel: img.Header.BuildIdentifier,
	}

	for i := range img.Partitions {
		p := &img.Partitions[i]

		logger.Info("Verifying image checksum",
			"imageName", p.Name,
			"hashAlgo", p.HashAlgo,
			"wantedHash", hex.EncodeToString(p.Hash))
		if err := img.VerifyHash(p.Name); err != nil {
			logger.Error("Checksum mismatch",
				"imageName", p.Name,
				"err", err)
			return nil, fmt.Errorf("%w: %s: %w", ErrChecksumMismatch, p.Name, err)
		}
		logger.Info("Image checksum OK",
			"imageName", p.Name)

		uncompressedSize, err := img.UncompressedSize(p.Name)
		if err != nil {
			return nil, err
		}

		partitionName := p.Name
		fw.Partitions = append(fw.Partitions, firmwareImagePartition{
			Name:             partitionName,
			Partition:        partitionName,
			UncompressedSize: uncompressedSize,
			Open: func() (io.ReadCloser, error) {
				return img.Open(partitionName)
			},
		})
	}

	return fw, nil
}

type Updater struct {
	config Config
	logger *slog.Logger

	DryRun bool
}

func NewUpdater(config Config, logger *slog.Logger) (*Updater, error) {
	if logger == nil {
		logger = slog.Default()
	}

	if len(config.Devices) < 1 {
		return nil, ErrUnsupportedConfiguration
	}

	return &Updater{
		config: config,
		logger: logger,
	}, nil
}

func (u Updater) Config() Config {
	return u.config
}

func (u Updater) runDevice(progressC chan Progress, deviceConfig DeviceConfig) error {
	progressC <- Progress{
		Text:         "Preparing update...",
		Indetermined: true,
	}

	imageFile, err := os.Open(deviceConfig.ImagePath)
	if err != nil {
		return err
	}
	defer imageFile.Close()

	fw, err := loadFirmwareImage(imageFile, u.logger)
	if err != nil {
		return err
	}

	var totalDataSizeFloat float64
	for _, part := range fw.Partitions {
		totalDataSizeFloat += float64(part.UncompressedSize)
	}

	u.logger.Info("Calculated total data length",
		"totalSize", int64(totalDataSizeFloat))

	usbCtx := gousb.NewContext()
	defer usbCtx.Close()

	usbCtx.Debug(u.config.LibusbDebugLevel)

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appCtx, cancelNotify := signal.NotifyContext(appCtx, os.Interrupt, syscall.SIGTERM)
	defer cancelNotify()

	devicesMatched := 0

	for _, deviceID := range fw.Devices {
		withDevice := func(f func(fb *fastboot.FastBootChannel) error) error {
			device, err := usbCtx.OpenDeviceWithVIDPID(
				gousb.ID(deviceID.VendorID),
				gousb.ID(deviceID.ProductID))
			if err != nil {
				if errors.Is(err, gousb.ErrorAccess) {
					dialog.Message(
						"Permission error. Make sure you are running the application with correct permissions (you may want to run this with admin privileges).\n"+
							"\n"+
							"%s",
						err.Error()).Title("Error").Error()
				}
				return err
			}
			if device == nil {
				return ErrNoMatchingDevices
			}
			defer device.Close()
			devicesMatched++

			u.logger.Debug("Enabling autodetach")
			device.SetAutoDetach(true)

			u.logger.Debug("Setting configuration...",
				"configNum", deviceConfig.USBConfig)
			cfg, err := device.Config(deviceConfig.USBConfig)
			if err != nil {
				return fmt.Errorf("dev.Config(%d): %w", deviceConfig.USBConfig, err)
			}
			u.logger.Debug("Claiming interface...",
				"interfaceNum", deviceConfig.USBInterface,
				"altNum", deviceConfig.USBAlternate)
			intf, err := cfg.Interface(deviceConfig.USBInterface, deviceConfig.USBAlternate)
			if err != nil {
				return fmt.Errorf("cfg.Interface(%d, %d): %w", deviceConfig.USBInterface, deviceConfig.USBAlternate, err)
			}
			defer intf.Close()

			u.logger.Debug("Using input endpoint",
				"inputEndpoint", deviceConfig.USBInputEndpoint)
			inEP, err := intf.InEndpoint(deviceConfig.USBInputEndpoint)
			if err != nil {
				return fmt.Errorf("dev.InEndpoint(): %w", err)
			}
			u.logger.Debug("Found input endpoint",
				"inEP", inEP)
			var rdr fastboot.ContextReader = inEP
			if deviceConfig.USBReadBufferSize > 1 {
				u.logger.Debug("Creating input buffer...")
				s, err := inEP.NewStream(deviceConfig.USBReadSize, deviceConfig.USBReadBufferSize)
				if err != nil {
					return fmt.Errorf("inEP.NewStream(): %w", err)
				}
				defer s.Close()
				rdr = s
			}

			u.logger.Debug("Using output endpoint",
				"outputEndpoint", deviceConfig.USBOutputEndpoint)
			outEP, err := intf.OutEndpoint(deviceConfig.USBOutputEndpoint)
			if err != nil {
				return fmt.Errorf("dev.OutEndpoint(): %w", err)
			}
			u.logger.Debug("Found input endpoint",
				"outEP", outEP)
			var wrr fastboot.ContextWriter = outEP
			if deviceConfig.USBWriteBufferSize > 1 {
				u.logger.Debug("Creating output buffer...")
				s, err := outEP.NewStream(deviceConfig.USBWriteSize, deviceConfig.USBWriteBufferSize)
				if err != nil {
					return fmt.Errorf("outEP.NewStream(): %w", err)
				}
				defer s.Close()
				wrr = s
			}

			fbCtx, cancelfb := context.WithCancel(appCtx)
			defer cancelfb()

			fb := fastboot.NewFastBootChannel(fbCtx,
				u.logger.WithGroup("fastboot"),
				rdr,
				wrr)

			bootloaderLog := u.logger.WithGroup("bootloader")
			go func() {
				for info := range fb.InfoC() {
					bootloaderLog.Info(info)
				}
			}()

			go func() {
				for text := range fb.TextC() {
					u.logger.Info(text)
				}
			}()

			return f(fb)
		}

		withTimeout := func(f func(opCtx context.Context)) {
			opCtx := appCtx
			if deviceConfig.USBOpTimeout > 0 {
				u.logger.Debug("Setting up deadline",
					"timeout", deviceConfig.USBOpTimeout)
				var cancelTimeout func()
				opCtx, cancelTimeout = context.WithTimeout(appCtx, deviceConfig.USBOpTimeout)
				defer cancelTimeout()
			}
			f(opCtx)
		}

		// unlock device for flashing
		//
		// OEM commands use the space-separated form "oem <magic>" (standard
		// fastboot OEM command convention), not "oem:<magic>".
		if err := withDevice(func(fb *fastboot.FastBootChannel) error {
			var err error
			withTimeout(func(opCtx context.Context) {
				_, err = fb.Command(opCtx, "oem %s", "inmusic-unlock-magic-7de5fbc22b8c524e")
				if err != nil {
					return
				}
			})
			return err
		}); err != nil {
			return err
		}

		// log some basic fastboot variables
		fields := make([]any, 0)
		for _, varName := range []string{
			"version",
			"version-bootloader",
			"version-baseband",
			"product",
			"serialno",
			"secure",
			"is-userspace",
			"max-download-size",
			"power-source",
		} {
			if err := withDevice(func(fb *fastboot.FastBootChannel) error {
				var data string
				var err error
				withTimeout(func(opCtx context.Context) {
					data, err = fb.GetVar(opCtx, varName)
				})
				if err != nil {
					u.logger.Warn("Bootloader does not support variable",
						"varName", varName)
					return nil
				}
				fields = append(fields, varName, data)
				return nil
			}); err != nil {
				return err
			}
		}
		u.logger.Info("Read bootloader variables", fields...)

		// download image to device
		var totalDownloadedSizeFloat float64
		statusText := fmt.Sprintf("Updating to version %s...", fw.VersionLabel)
		for _, part := range fw.Partitions {
			u.logger.Info("Parsing image data",
				"imageName", part.Name)

			dataSize := part.UncompressedSize

			finalReader, err := part.Open()
			if err != nil {
				return err
			}
			defer finalReader.Close()

			u.logger.Info("Now writing image",
				"imageName", part.Name,
				"partition", part.Partition,
				"dataSize", dataSize)

			// monitor our progress on the decoded data
			var previousDataPos int64
			monitoredReader := NewReaderMonitor(finalReader, func(offset int64) {
				// calculate pos difference and then store new pos
				newDataPos := previousDataPos + offset
				dataPosDiff := newDataPos - previousDataPos
				previousDataPos = newDataPos

				// add difference to TOTAL size for total progress
				totalDownloadedSizeFloat += float64(dataPosDiff)

				progressC <- Progress{
					Text: statusText + fmt.Sprintf("\n(%s, transferred %s/%s)",
						part.Name,
						humanize.Bytes(uint64(newDataPos)),
						humanize.Bytes(uint64(dataSize))),
					Percentage: totalDownloadedSizeFloat / totalDataSizeFloat,
				}
			})

			if err := withDevice(func(fb *fastboot.FastBootChannel) error {
				u.logger.Info("Download started",
					"dataSize", dataSize,
					"imageName", part.Name,
					"dryRun", u.DryRun)
				if u.DryRun {
					io.Copy(io.Discard, monitoredReader)
				} else if err := fb.DownloadFromReader(appCtx, monitoredReader, uint32(dataSize)); err != nil {
					u.logger.Error("Download failed",
						"err", err)
					return fmt.Errorf("download failed: %w", err)
				}
				u.logger.Info("Download OK")
				return nil
			}); err != nil {
				return err
			}

			progressC <- Progress{
				Text: statusText + fmt.Sprintf("\n(%s, flashing)",
					part.Name),
				Percentage: totalDownloadedSizeFloat / totalDataSizeFloat,
			}
			if err := withDevice(func(fb *fastboot.FastBootChannel) error {
				u.logger.Info("Flash started",
					"imageName", part.Name,
					"dryRun", u.DryRun)
				if u.DryRun {
					time.Sleep(2 * time.Second)
				} else {
					if err := fb.Flash(appCtx, part.Partition); err != nil {
						u.logger.Error("Flash failed",
							"err", err)
						return fmt.Errorf("flash failed: %w", err)
					}
				}
				u.logger.Info("Flash OK")
				return nil
			}); err != nil {
				return err
			}
			time.Sleep(1 * time.Second)
		}

		progressC <- Progress{
			Text:         "Finishing update...",
			Indetermined: true,
		}
		if !u.config.SkipRebootAfterFlash {
			if err := withDevice(func(fb *fastboot.FastBootChannel) error {
				u.logger.Info("Requesting reboot",
					"dryRun", u.DryRun)
				if !u.DryRun {
					if err := fb.Reboot(appCtx); err != nil {
						u.logger.Error("Reboot failed", "err", err)
						return fmt.Errorf("reboot failed: %w", err)
					}
				}
				u.logger.Info("Reboot OK")
				return nil
			}); err != nil {
				return err
			}
		}
		time.Sleep(1 * time.Second)
	}

	if devicesMatched == 0 {
		return ErrNoMatchingDevices
	}

	return nil
}

func (u Updater) Run(progressC chan Progress) error {
	defer close(progressC)

	config := u.config

	if len(config.Devices) < 1 {
		return errors.New("configurations with not exactly 1 device not supported yet")
	}

	return u.runDevice(progressC, config.Devices[0])
}
