// Command updater-cli flashes a firmware image over USB without the GUI.
//
// Output adapts to where it goes: a terminal gets one status line redrawn in
// place, a pipe or log file gets one line per phase plus percentage steps. Both
// stay line-oriented so a script can read them.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/icedream/denon-prime4/go/pkg/updater"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var (
	flagConfig      = flag.String("config", "config.toml", "config file holding the device entry and image path")
	flagDryRun      = flag.Bool("dry", false, "talk to the device without flashing it")
	flagSkipReboot  = flag.Bool("skip_reboot", false, "leave the device in fastboot mode when done")
	flagDebugLevel  = flag.Int("debug", int(slog.LevelError), "log level, 0 debug through 4 error")
	flagLibusbDebug = flag.Int("libusb_debug", 0, "libusb debug level, 0 through 3")
)

const spinner = `|/-\`

func main() {
	flag.Parse()

	config, err := loadConfig(*flagConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "updater-cli: %v\n", err)
		os.Exit(1)
	}

	config.LibusbDebugLevel = *flagLibusbDebug
	config.SkipRebootAfterFlash = *flagSkipReboot

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.Level(*flagDebugLevel),
	}))

	instance, err := updater.NewUpdater(config, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "updater-cli: %v\n", err)
		os.Exit(1)
	}
	instance.DryRun = *flagDryRun

	progress := make(chan updater.Progress)
	rendered := make(chan struct{})
	go renderProgress(progress, os.Stdout, rendered)

	runErr := instance.Run(progress)
	<-rendered

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "updater-cli: %v\n", runErr)
		os.Exit(1)
	}
	fmt.Println("flash complete")
}

func loadConfig(path string) (updater.Config, error) {
	var config updater.Config

	ko := koanf.New(".")
	if err := ko.Load(file.Provider(path), toml.Parser()); err != nil {
		return config, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := ko.Unmarshal("", &config); err != nil {
		return config, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(config.Devices) == 0 {
		return config, fmt.Errorf("%s declares no devices", path)
	}
	if _, err := os.Stat(config.Devices[0].ImagePath); err != nil {
		return config, fmt.Errorf("image for %s: %w", config.Devices[0].Name, err)
	}

	return config, nil
}

func renderProgress(progress <-chan updater.Progress, out *os.File, done chan<- struct{}) {
	defer close(done)

	tty := isTerminal(out)
	started := time.Now()
	phase := ""
	reported := -1.0
	frame := 0
	current := updater.Progress{}
	haveCurrent := false

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case p, ok := <-progress:
			if !ok {
				if tty {
					fmt.Fprintln(out)
				}
				return
			}

			current, haveCurrent = p, true

			if p.Text != phase {
				phase, reported = p.Text, -1.0
				if tty {
					fmt.Fprintln(out)
				}
				fmt.Fprintf(out, "%s\n", phase)
			}

			switch {
			case tty:
				drawTTY(out, p, started, frame)
			case !p.Indetermined && (p.Percentage-reported >= 0.25 || p.Percentage >= 0.995):
				fmt.Fprintf(out, "  %5.1f%%\n", p.Percentage*100)
				reported = p.Percentage
			}

		case <-ticker.C:
			if tty && haveCurrent {
				frame++
				drawTTY(out, current, started, frame)
			}
		}
	}
}

func drawTTY(out *os.File, p updater.Progress, started time.Time, frame int) {
	elapsed := time.Since(started).Truncate(100 * time.Millisecond)

	if p.Indetermined {
		fmt.Fprintf(out, "\r%c %s (%s)\x1b[K", spinner[frame%len(spinner)], p.Text, elapsed)
		return
	}

	share := p.Percentage
	if share < 0 {
		share = 0
	}
	if share > 1 {
		share = 1
	}

	width := barWidth()
	filled := int(share*float64(width) + 0.5)
	bar := strings.Repeat("#", filled) + strings.Repeat("-", width-filled)
	fmt.Fprintf(out, "\r[%s] %5.1f%% %s (%s)\x1b[K", bar, share*100, p.Text, elapsed)
}

func barWidth() int {
	width := 40
	if columns, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && columns > 0 {
		width = columns - 30
	}
	if width < 10 {
		width = 10
	}
	if width > 60 {
		width = 60
	}
	return width
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
