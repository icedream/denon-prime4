package updater

import "time"

type DeviceConfig struct {
	Name                string
	ProductID, VendorID uint16
	ImagePath           string

	USBConfig    int
	USBInterface int
	USBAlternate int

	USBInputEndpoint  int
	USBReadSize       int
	USBReadBufferSize int

	USBOutputEndpoint  int
	USBWriteSize       int
	USBWriteBufferSize int

	USBOpTimeout time.Duration
}

type Config struct {
	Devices []DeviceConfig

	LibusbDebugLevel     int
	SkipRebootAfterFlash bool
}
