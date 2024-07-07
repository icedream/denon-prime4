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
	fdt, err := dt.New(dt.WithReaderAt(imageFile))
	if err != nil {
		return err
	}

	// extract list of compatible devices
	devices := fdt.Root().Property("inmusic,devices")
	devicesBytes, err := devices.AsBytes()
	if err != nil {
		return err
	}
	devicesList, err := bytesAsDeviceList(devicesBytes)
	if err != nil {
		return err
	}

	// extract version string
	version := fdt.Root().Property("inmusic,version")
	if version == nil {
		return ErrMissingVersion
	}
	versionStr, err := version.AsString()
	if err != nil {
		return ErrBadVersion
	}

	images := fdt.Root().Walk("images")
	if images == nil {
		return ErrNoImagesInDeviceTree
	}
	var totalDataSizeFloat float64
	imageNames, err := images.ListChildNodes()
	if err != nil {
		return err
	}
	for _, imageName := range imageNames {
		image := images.Walk(imageName)
		if image == nil {
			continue
		}

		// image
		data := image.Property("data")
		if data == nil {
			return err
		}
		dataBytes, err := data.AsBytes()
		if err != nil {
			return err
		}
		compression := image.Property("compression")
		var compressionStr string
		dataSize := float64(len(dataBytes))
		dataReader := bytes.NewReader(dataBytes)
		if compression != nil {
			compressionStr, err = compression.AsString()
			if err != nil {
				return err
			}
			switch compressionStr {
			case "xz":
				// determine uncompressed size
				uncompressedSize, err := getXZUncompressedLength(bytes.NewReader(dataBytes))
				if err != nil {
					return err
				}
				dataSize = float64(uncompressedSize)
			default:
				u.logger.Error("Unsupported compression",
					"compression", compressionStr,
					"imageName", imageName)
				return errors.New("unsupported compression: " + compressionStr)
			}
		}
		totalDataSizeFloat += dataSize

		// verify image hash
		hashProp := image.Walk("hash")
		if hashProp == nil {
			continue
		}
		hashAlgo := hashProp.Property("algo")
		if hashAlgo != nil {
			hashAlgoStr, err := hashAlgo.AsString()
			if err != nil {
				return err
			}
			hashValue := hashProp.Property("value")
			if hashValue == nil {
				return err
			}
			hashBytes, err := hashValue.AsBytes()
			if err != nil {
				return err
			}
			var hasher hash.Hash
			switch hashAlgoStr {
			case "sha1":
				hasher = sha1.New()
			default:
				u.logger.Error("Checksum algorithm not supported",
					"imageName", imageName,
					"hashAlgo", hashAlgoStr)
				return errors.New("checksum algorithm not supported yet: " + hashAlgoStr)
			}
			u.logger.Info("Verifying image checksum",
				"imageName", imageName,
				"hashAlgo", hashAlgoStr,
				"wantedHash", hex.EncodeToString(hashBytes))
			if _, err := io.Copy(hasher, dataReader); err != nil {
				u.logger.Error("Failed to generate checksum",
					"imageName", imageName,
					"err", err)
				// TODO - ErrChecksumGenerationFailure
				return fmt.Errorf("checksum generation failure: %w", err)
			}
			actualHash := hasher.Sum(nil)
			if !bytes.Equal(actualHash, hashBytes) {
				u.logger.Error("Checksum mismatch",
					"imageName", imageName,
					"hashAlgo", hashAlgoStr,
					"wantedHash", hex.EncodeToString(hashBytes),
					"actualHash", hex.EncodeToString(actualHash))
				return ErrChecksumMismatch
			}
			u.logger.Info("Image checksum OK",
				"imageName", imageName)
		} else {
			return errors.New("missing image hash")
		}
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

	for _, deviceID := range devicesList {
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
		if err := withDevice(func(fb *fastboot.FastBootChannel) error {
			var err error
			withTimeout(func(opCtx context.Context) {
				_, err = fb.Command(opCtx, "oem:%s", "inmusic-unlock-magic-7de5fbc22b8c524e")
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
		statusText := fmt.Sprintf("Updating to version %s...", versionStr)
		for _, imageName := range imageNames {
			u.logger.Info("Parsing image data",
				"imageName", imageName)

			image := images.Walk(imageName)
			if image == nil {
				continue
			}

			// parse partition
			partition := image.Property("partition")
			if partition == nil {
				return errors.New("missing partition")
			}
			partitionStr, err := partition.AsString()
			if err != nil {
				return err
			}

			// parse data and data size
			data := image.Property("data")
			if data == nil {
				return errors.New("missing data")
			}
			dataBytes, err := data.AsBytes()
			if err != nil {
				return err
			}
			dataSize := int64(len(dataBytes))
			compressedDataSize := dataSize
			dataReader := bytes.NewReader(dataBytes)

			var finalReader io.Reader = dataReader
			compression := image.Property("compression")
			var compressionStr string
			if compression != nil {
				compressionStr, err = compression.AsString()
				if err != nil {
					return err
				}
				switch compressionStr {
				case "xz":
					// determine uncompressed size
					uncompressedSize, err := getXZUncompressedLength(bytes.NewReader(dataBytes))
					if err != nil {
						return err
					}
					dataSize = uncompressedSize

					// decompress on the fly
					uncompressedDataReader, err := newXZReader(finalReader)
					if err != nil {
						return err
					}
					finalReader = uncompressedDataReader
				default:
					u.logger.Error("Unsupported compression",
						"compression", compressionStr,
						"imageName", imageName)
					return errors.New("unsupported compression: " + compressionStr)
				}
			}

			u.logger.Info("Now writing image",
				"imageName", imageName,
				"partition", partitionStr,
				"compressedDataSize", compressedDataSize,
				"dataSize", dataSize,
				"compression", compressionStr)

			// monitor our progress on the decoded data
			var previousDataPos int64
			// var countedPos int64
			// finalReader = NewReadSeekerMonitor(finalReader, func(offset int64, whence int) {
			// 	var newPos int64
			// 	switch whence {
			// 	case io.SeekCurrent:
			// 		newPos = previousPos + offset
			// 	case io.SeekEnd:
			// 		newPos = int64(len(dataBytes))
			// 		if offset < 0 {
			// 			newPos += offset
			// 		}
			// 	case io.SeekStart:
			// 		newPos = offset
			// 	}
			// 	defer func() { previousPos = newPos }()
			// 	if countedPos >= newPos {
			// 		return
			// 	}
			// 	diff := newPos - previousPos
			// 	countedPos = newPos
			// 	downloadedSize += float64(diff)
			// 	progressC <- Progress{
			// 		Text: statusText + fmt.Sprintf("\n(%s, transferred %s/%s)",
			// 			imageName,
			// 			humanize.Bytes(uint64(newPos)),
			// 			humanize.Bytes(uint64(len(dataBytes)))),
			// 		Percentage: downloadedSize / totalSize,
			// 	}
			// })
			finalReader = NewReaderMonitor(finalReader, func(offset int64) {
				// calculate pos difference and then store new pos
				newDataPos := previousDataPos + offset
				dataPosDiff := newDataPos - previousDataPos
				previousDataPos = newDataPos

				// add difference to TOTAL size for total progress
				totalDownloadedSizeFloat += float64(dataPosDiff)

				progressC <- Progress{
					Text: statusText + fmt.Sprintf("\n(%s, transferred %s/%s)",
						imageName,
						humanize.Bytes(uint64(newDataPos)),
						humanize.Bytes(uint64(dataSize))),
					Percentage: totalDownloadedSizeFloat / totalDataSizeFloat,
				}
			})

			// buf := make([]byte, 4096)
			if err := withDevice(func(fb *fastboot.FastBootChannel) error {
				u.logger.Info("Download started",
					"compressedDataSize", compressedDataSize,
					"dataSize", dataSize,
					"imageName", imageName,
					"dryRun", u.DryRun)
				if u.DryRun {
					io.Copy(io.Discard, finalReader)
				} else if err := fb.DownloadFromReader(appCtx, finalReader, uint32(dataSize)); err != nil {
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
					imageName),
				Percentage: totalDownloadedSizeFloat / totalDataSizeFloat,
			}
			if err := withDevice(func(fb *fastboot.FastBootChannel) error {
				u.logger.Info("Flash started",
					"imageName", imageName,
					"dryRun", u.DryRun)
				if u.DryRun {
					time.Sleep(2 * time.Second)
				} else {
					if err := fb.Flash(appCtx, partitionStr); err != nil {
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
