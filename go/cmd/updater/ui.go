package main

import (
	"context"
	"errors"
	"image"
	"image/color"
	"log"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/google/gousb"
	"github.com/icedream/denon-prime4/go/pkg/fastboot"
	"github.com/icedream/denon-prime4/go/pkg/updater"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/roblillack/spot/ui"
)

func doLayout(graphicsCtx layout.Context, state *State) {
	theme := material.NewTheme()
	// Flip to the dark side
	palettedTheme := theme.WithPalette(material.Palette{
		Bg: color.NRGBA{0x1a, 0x1a, 0x1a, 0xff},
		Fg: color.NRGBA{0xff, 0xff, 0xff, 0xff},

		ContrastBg: color.NRGBA{0x11, 0xaa, 0x33, 0xff},
		ContrastFg: color.NRGBA{0xff, 0xff, 0xff, 0xff},
	})
	theme = &palettedTheme

	// Font
	fontCollection := gofont.Collection()
	theme.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(fontCollection))

	// set the background color
	macro := op.Record(graphicsCtx.Ops)
	rect := image.Rectangle{
		Max: image.Point{
			X: graphicsCtx.Constraints.Max.X,
			Y: graphicsCtx.Constraints.Max.Y,
		},
	}
	paint.FillShape(graphicsCtx.Ops, theme.Palette.Bg, clip.Rect(rect).Op())
	background := macro.Stop()

	background.Add(graphicsCtx.Ops)

	flex := layout.Flex{
		// Vertical alignment, from top to bottom
		Axis: layout.Vertical,
		// Empty space is left at the start, i.e. at the top
		Spacing: layout.SpaceAround,
	}
	flex.Layout(
		graphicsCtx,
		// logo
		layout.Rigid(
			func(gtx layout.Context) layout.Dimensions {
				margins := layout.Inset{
					Top:    unit.Dp(10),
					Bottom: unit.Dp(10),
					Left:   unit.Dp(10),
					Right:  unit.Dp(10),
				}
				return margins.Layout(
					gtx,
					// The height of the spacer is 25 Device independent pixels
					layout.Spacer{Height: unit.Dp(25)}.Layout,
				)
			},
		),
		// title
		layout.Rigid(
			func(gtx layout.Context) layout.Dimensions {
				margins := layout.Inset{
					Top:    unit.Dp(10),
					Bottom: unit.Dp(10),
					Left:   unit.Dp(10),
					Right:  unit.Dp(10),
				}
				return margins.Layout(
					gtx,
					func(gtx layout.Context) layout.Dimensions {
						title := material.H2(theme, "Firmware updater")
						title.Alignment = text.Middle
						return title.Layout(gtx)
					},
				)
			},
		),
		// list of supported devices for this updater
		layout.Rigid(
			func(gtx layout.Context) layout.Dimensions {
				margins := layout.Inset{
					Top:    unit.Dp(10),
					Bottom: unit.Dp(10),
					Left:   unit.Dp(10),
					Right:  unit.Dp(10),
				}
				return margins.Layout(
					gtx,
					func(gtx layout.Context) layout.Dimensions {
						txt := ""
						for _, device := range state.Devices {
							txt += device.Name + "\n"
						}

						deviceTitle := material.Body1(theme, txt)
						deviceTitle.Alignment = text.Middle
						return deviceTitle.Layout(gtx)
					},
				)
			},
		),
		// progress panel
		layout.Flexed(
			0.2,
			func(gtx layout.Context) layout.Dimensions {
				margins := layout.Inset{
					Top:    unit.Dp(10),
					Bottom: unit.Dp(10),
					Left:   unit.Dp(10),
					Right:  unit.Dp(10),
				}
				return margins.Layout(
					gtx,
					func(gtx layout.Context) layout.Dimensions {
						children := []layout.FlexChild{}

						if state.isFlashRunning {
							children = append(children,
								layout.Rigid(
									func(gtx layout.Context) layout.Dimensions {
										statusBody := material.Body2(theme, "")
										if state.flashProgress != nil {
											statusBody.Text = state.flashProgress.Text
										}
										statusBody.Alignment = text.Middle
										return statusBody.Layout(gtx)
									},
								),
							)
						} else {
							children = append(children,
								layout.Rigid(
									func(gtx layout.Context) layout.Dimensions {
										statusBody := material.Body2(theme, state.finalProgressText)
										statusBody.Alignment = text.Middle
										if state.isFlashFailed {
											statusBody.Color = color.NRGBA{0xaa, 0x22, 0x22, 0xff}
										} else if state.isFlashDone {
											statusBody.Color = color.NRGBA{0x11, 0xaa, 0x33, 0xff}
										}
										return statusBody.Layout(gtx)
									},
								),
							)
						}

						if state.flashProgress != nil && !state.flashProgress.Indetermined {
							children = append(children,
								layout.Rigid(
									func(gtx layout.Context) layout.Dimensions {
										progressBar := material.ProgressBar(theme, 0)
										progressBar.Height = theme.FingerSize
										progressBar.Radius = 2
										if state.flashProgress != nil &&
											!state.flashProgress.Indetermined {
											progressBar.Progress = float32(state.flashProgress.Percentage)
										}
										return progressBar.Layout(gtx)
									},
								),
							)
						} else {
							children = append(children,
								layout.Rigid(
									func(gtx layout.Context) layout.Dimensions {
										// disable the button if a flash is already running
										if state.isFlashRunning {
											gtx = gtx.Disabled()
										}

										btn := material.Button(theme, &state.startButton, "START UPDATE")
										btn.Font.Weight = font.Bold
										if state.flashProgress != nil {
											btn.Text = "UPDATING..."
											btn.Background = color.NRGBA{0x00, 0x00, 0x00, 0x7f}
										} else {
											btn.Background = color.NRGBA{0x11, 0xaa, 0x33, 0xff}
											btn.Color = color.NRGBA{0xff, 0xff, 0xff, 0xff}
										}
										return btn.Layout(gtx)
									},
								),
							)
						}

						// render current progress instead
						return layout.Flex{
							Axis:    layout.Vertical,
							Spacing: layout.SpaceStart,
						}.Layout(gtx, children...)
					},
				)
			},
		),
	)
}

type State struct {
	window *app.Window

	ops               op.Ops
	startButton       widget.Clickable
	flashProgress     *updater.Progress
	finalProgressText string
	isFlashDone       bool
	isFlashFailed     bool
	isFlashRunning    bool

	Devices []updater.DeviceConfig
}

type Err struct {
	Err     error
	ErrType reflect.Type
}

func errChain(err error) []Err {
	errs := []Err{}
	for unwrappedErr := err; unwrappedErr != nil; unwrappedErr = errors.Unwrap(unwrappedErr) {
		errs = append(errs, Err{Err: unwrappedErr, ErrType: reflect.TypeOf(unwrappedErr)})
	}
	return errs
}

const (
	corruptedMessage      = "Please redownload this software."
	maybeCorruptedMessage = "Make sure your copy of this software has not been corrupted."
	closeAppsMessage      = "Make sure to close any application that may interact with the device, reconnect it and retry the update."
	runAdminMessage       = "Please run this app with higher privileges (e.g. as administrator)."
	retryMessage          = "Please reconnect the device and retry the update."
)

func buildMessage(messages ...string) string {
	return strings.Join(messages, " ")
}

func triggerUpdate(u *updater.Updater, state *State) {
	window := state.window

	state.isFlashDone = false
	state.isFlashFailed = false
	state.finalProgressText = ""
	state.flashProgress = nil
	state.isFlashRunning = true
	defer func() {
		state.isFlashRunning = false
	}()

	progressC := make(chan updater.Progress, 1)
	go func() {
		ticker := time.NewTicker(8 * time.Millisecond)
		defer func() {
			ticker.Stop()
			window.Invalidate()
		}()
		for {
			select {
			case progress, ok := <-progressC:
				if !ok {
					return
				}
				state.flashProgress = &progress
			case <-ticker.C:
				window.Invalidate()
			}
		}
	}()

	err := u.Run(progressC)
	state.isFlashDone = true
	state.flashProgress = nil
	if err != nil {
		log.Println("Update failed:", err)
		state.isFlashFailed = true
		message := err.Error()
		var usbTransferStatus gousb.TransferStatus
		switch {
		case errors.Is(err, gousb.ErrorAccess):
			message = buildMessage("Permission denied.", runAdminMessage)
		case errors.Is(err, gousb.ErrorBusy):
			message = buildMessage("Device is busy.", closeAppsMessage)
		case errors.Is(err, gousb.ErrorInterrupted):
			message = buildMessage("Communication with the device was interrupted.", retryMessage)
		case errors.Is(err, gousb.ErrorNoDevice):
			message = buildMessage("Device is no longer present.", retryMessage)
		case errors.Is(err, gousb.ErrorNotFound):
			message = buildMessage("Device was not found.", retryMessage)
		case errors.Is(err, gousb.ErrorNotSupported):
			message = buildMessage("An operation necessary for the update process is not supported.")
		case errors.Is(err, gousb.ErrorTimeout),
			errors.Is(err, gousb.TransferTimedOut),
			errors.Is(err, context.Canceled):
			message = buildMessage("Communication with the device was canceled or has timed out.", retryMessage)
		case errors.As(err, &usbTransferStatus):
			switch usbTransferStatus {
			case gousb.TransferError:
				message = "USB transfer failed."
			case gousb.TransferTimedOut:
				message = "USB transfer timed out."
			case gousb.TransferStall:
				message = "Communication with the device was halted."
			case gousb.TransferNoDevice:
				message = "Communication with the device was lost."
			default:
				message = "USB transfer entered an unexpected state."
			}
			message = buildMessage(message, retryMessage)
		case errors.Is(err, fastboot.ErrUnexpectedResponse):
			message = buildMessage("An unexpected response was sent by the device.", retryMessage)
		case errors.Is(err, updater.ErrNoImagesInDeviceTree):
			message = buildMessage("Firmware update does not contain any flashable images.", maybeCorruptedMessage)
		case errors.Is(err, updater.ErrBadVersion):
			message = buildMessage("Firmware update does not contain valid version information.", maybeCorruptedMessage)
		case errors.Is(err, updater.ErrChecksumMismatch):
			message = buildMessage("The firmware update seems to have been corrupted.", corruptedMessage)
		case errors.Is(err, updater.ErrFooterMagicMismatch):
			message = buildMessage("Part of the firmware update seems to have been corrupted.", corruptedMessage)
		case errors.Is(err, updater.ErrMissingVersion):
			message = buildMessage("Firmware update does not contain version information.", corruptedMessage)
		case errors.Is(err, updater.ErrNoMatchingDevices):
			message = "No matching devices were found. Please plug in one of the devices listed above and reboot it into bootloader mode. Check the manual for instructions."
		case errors.Is(err, updater.ErrUnsupportedConfiguration):
			message = "Unsupported configuration."
		default:
			slog.Warn("Unknown error type",
				"err", err,
				"errType", reflect.TypeOf(err),
				"errChain", errChain(err))
		}
		state.finalProgressText = "Update failed.\n\n" + message
	} else {
		state.finalProgressText = "Update succeeded."
	}
	window.Invalidate()
}

func runUI(u *updater.Updater, window *app.Window) error {
	var state State
	state.window = window
	state.Devices = u.Config().Devices
	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			return event.Err
		case app.FrameEvent:
			// This graphics context is used for managing the rendering state.
			graphicsCtx := app.NewContext(&state.ops, event)

			if state.startButton.Clicked(graphicsCtx) && !state.isFlashRunning {
				go triggerUpdate(u, &state)
			}

			doLayout(graphicsCtx, &state)

			// Pass the drawing operations to the GPU.
			event.Frame(graphicsCtx.Ops)
		}
	}
}

func initUI() error {
	ui.Init()

	// Load config
	k := koanf.New(".")
	parser := toml.Parser()
	if err := k.Load(file.Provider("config.toml"), parser); err != nil {
		log.Fatalf("error loading config: %v", err)
	}
	var config updater.Config
	if err := k.Unmarshal("", &config); err != nil {
		return err
	}

	programLevel := new(slog.LevelVar) // Info by default
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: programLevel,
	}))
	slog.SetDefault(logger)

	// set log level if requested
	if flagDebugLevel != nil {
		programLevel.Set(slog.Level(*flagDebugLevel))
	}

	// set libusb debug level if requested
	if flagLibusbDebugLevel != nil {
		config.LibusbDebugLevel = *flagLibusbDebugLevel
	}

	if flagSkipRebootAfterFlash != nil {
		config.SkipRebootAfterFlash = *flagSkipRebootAfterFlash
	}

	updater, err := updater.NewUpdater(config, logger)
	if err != nil {
		logger.Error("Failed to initialize updater",
			"err", err)
	}
	if flagDryRun != nil {
		updater.DryRun = *flagDryRun
	}

	go func() {
		window := new(app.Window)
		window.Option(
			app.Title("Updater"),
			app.Size(unit.Dp(550), unit.Dp(400)),
			app.MinSize(unit.Dp(500), unit.Dp(330)),
		)
		err := runUI(updater, window)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
	return nil
}
