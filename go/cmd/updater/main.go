package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
)

var (
	flagDebugLevel           = flag.Int("debug", int(slog.LevelError), fmt.Sprintf("logging level (ranges from %d for debug to %d for error)", int(slog.LevelDebug), int(slog.LevelError)))
	flagLibusbDebugLevel     = flag.Int("libusb_debug", 0, fmt.Sprintf("libusb debug level (%d..%d)", 0, 3))
	flagSkipRebootAfterFlash = flag.Bool("skip_reboot", false, "Whether to skip reboot after flashing")
	flagDryRun               = flag.Bool("dry", false, "Enable dry run (device still needs to be plugged in but will not actually flash)")
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	flag.Parse()

	if err := initUI(); err != nil {
		panic(err)
	}

	return nil
}
