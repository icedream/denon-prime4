# Computer Mode - Startup Sequence

## Entry

The device boots Engine OS normally. The user switches to Computer Mode via the
touchscreen UI. Engine then exits with reason `"ControllerMode"`.

## engine.sh (excerpt - ControllerMode case)

```sh
"ControllerMode")
    echo "Starting controller mode."
    /usr/bin/az01-ethernet-speed 1000
    touch /tmp/remote-screen-started
    /usr/Engine/Scripts/remote-screen.sh $APPNAME
    break
;;
```

Key steps:
1. `az01-ethernet-speed 1000` - sets internal Ethernet to 1 Gbit (RK3288 ↔ internal switch)
2. Creates `/tmp/remote-screen-started` sentinel file
3. Runs `remote-screen.sh`

When `engine.sh` restarts and finds `/tmp/remote-screen-started`, it waits 5s
for USB devices to re-enumerate before starting Engine again.

Engine also calls `az01-usbmux-switch external` then `internal` on exit code 7,
resetting the USB mux so the gadget re-enumerates on the host.

## remote-screen.sh

```sh
#!/bin/sh
APPNAME=$1

while [ 1 ]
do
    /usr/bin/planck-remote-screen
    rsAppExitCode=$?

    if [ "$rsAppExitCode" -eq "0" ]; then
        echo "Remote Screen App exited"
        /bin/az01-usbmux-switch internal
        systemctl restart engine
        break
    fi
    echo "Remote Screen App crashed ($rsAppExitCode)"
done
```

- If `planck-remote-screen` exits **cleanly (0)** - user requested to return
  to standalone → switches USB mux back and restarts Engine
- If it **crashes (non-zero)** - restart loop continues

## planck-remote-screen

Binary: `/usr/bin/planck-remote-screen` (ARM32 ELF, 11 MB, built with JUCE)

On startup:
1. Configures the USB gadget (`smexstream`) via `libusbgx` + configfs
2. Creates FunctionFS mount for the Screen interface
3. Waits for host (PC) to connect
4. Runs the `RemoteScreenApp` main loop:
   - `RemoteScreenNotConnectedComponent` shown until host connects
   - `RemoteScreenConnectedComponent` shown while connected
   - Receives `MessageBlock` packets from host via `UsbGadgetMessageBlockStream`
   - Renders images to local framebuffer (`/dev/fb0`) and display (`/dev/dri/...`)
   - Handles SMEX control messages (`smex.*`)
   - Forwards MIDI to/from control surface

## USB mux

The device has an `az01-usbmux-switch` utility that physically switches
the USB port between internal (standalone Engine) and external (Computer Mode
gadget) operation. This is why the USB devices disappear and reappear when
switching modes.

## Framebuffer

The Prime 4 main screen (800×1280 MIPI-DSI) is accessible from Engine OS as:
- `/dev/fb0` - framebuffer
- `/dev/dri/by-path/platform-ffa30000.gpu-render` - DRM render node (Mali GPU)

`planck-remote-screen` renders the Computer Mode UI directly to this framebuffer,
independent of what the host sends. The host sends **content data** (track info,
images), not raw pixels.
