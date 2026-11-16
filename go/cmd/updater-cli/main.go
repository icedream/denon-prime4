// Command updater-cli flashes a firmware image over USB without the GUI.
//
// Output adapts to where it goes: a terminal gets one bar per phase, the current
// one redrawn in place, while a pipe or log file gets one line per phase plus
// percentage steps.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/icedream/denon-prime4/go/pkg/updater"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

var (
	flagConfig      = flag.String("config", "config.toml", "config file holding the device entry and image path")
	flagDryRun      = flag.Bool("dry", false, "talk to the device without flashing it")
	flagSkipReboot  = flag.Bool("skip_reboot", false, "leave the device in fastboot mode when done")
	flagDebugLevel  = flag.Int("debug", int(slog.LevelError), "log level, 0 debug through 4 error")
	flagLibusbDebug = flag.Int("libusb_debug", 0, "libusb debug level, 0 through 3")
)

// progress arrives as a fraction, so bars work against a fixed scale.
const scale = 1000

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

	if isTerminal(out) {
		renderTTY(progress, out)
		return
	}
	renderLinear(progress, out)
}

// renderTTY keeps every finished phase on screen as a completed line and
// redraws only the phase in progress.
func renderTTY(progress <-chan updater.Progress, out *os.File) {
	container := mpb.New(mpb.WithOutput(out), mpb.WithRefreshRate(200*time.Millisecond))

	var current *mpb.Bar
	phase := ""

	for p := range progress {
		if p.Text != phase || current == nil {
			if current != nil {
				current.SetTotal(current.Current(), true)
			}
			current, phase = addBar(container, p), p.Text
		}
		if !p.Indetermined {
			current.SetCurrent(int64(p.Percentage * scale))
		}
	}

	if current != nil {
		current.SetTotal(current.Current(), true)
	}
	container.Wait()
}

func addBar(container *mpb.Progress, p updater.Progress) *mpb.Bar {
	name := mpb.PrependDecorators(decor.Name(p.Text, decor.WC{W: 24, C: decor.DindentRight}))

	if p.Indetermined {
		return container.AddSpinner(0, name,
			mpb.AppendDecorators(decor.Elapsed(decor.ET_STYLE_MMSS, decor.WC{W: 6})))
	}

	return container.AddBar(scale, name,
		mpb.AppendDecorators(
			decor.Percentage(decor.WC{W: 7}),
			decor.Elapsed(decor.ET_STYLE_MMSS, decor.WC{W: 6}),
		))
}

func renderLinear(progress <-chan updater.Progress, out *os.File) {
	phase := ""
	reported := -1.0

	for p := range progress {
		if p.Text != phase {
			phase, reported = p.Text, -1.0
			fmt.Fprintf(out, "%s\n", phase)
		}
		if p.Indetermined {
			continue
		}
		if p.Percentage-reported >= 0.25 || p.Percentage >= 0.995 {
			fmt.Fprintf(out, "  %5.1f%%\n", p.Percentage*100)
			reported = p.Percentage
		}
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
