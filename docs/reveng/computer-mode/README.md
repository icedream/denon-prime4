# Computer Mode - Overview

This directory documents the Denon Prime 4's **USB Computer Mode** protocol,
reverse-engineered from:

- `planck-remote-screen` binary in the JP11 firmware rootfs (ARM32 ELF)
- QML assignment files in the JP11 rootfs (`/usr/Engine/AssignmentFiles/`)
- Live USB enumeration with `lsusb` while the device is connected via USB/IP

## Documents

| File | Contents |
|---|---|
| [01-usb-devices.md](01-usb-devices.md) | USB device layout - all 5 USB devices the Prime 4 exposes |
| [02-startup-sequence.md](02-startup-sequence.md) | How computer mode is entered; engine.sh / remote-screen.sh flow |
| [03-jog-wheel-sysex.md](03-jog-wheel-sysex.md) | Jog wheel OLED display protocol (SysEx over MIDI) |
| [04-main-screen-smex.md](04-main-screen-smex.md) | Main 7" screen protocol (SMEX/MessageBlock over FunctionFS bulk) |
| [05-statemap-keys.md](05-statemap-keys.md) | StateMap property paths used by display assignments |

## Status

| Protocol | Status |
|---|---|
| USB device enumeration | complete |
| Computer Mode entry sequence | complete |
| Jog wheel SysEx commands | complete (from QML source) |
| Jog wheel image format | complete (198x198 PNG, SysEx-chunked) |
| Platter position MIDI | complete |
| Main screen USB transport | identified (FunctionFS, EP2 OUT bulk) |
| SMEX control message names | complete (from planck-remote-screen strings) |
| MessageBlock wire format | incomplete - needs further analysis of planck-remote-screen |
