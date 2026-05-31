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
| Main screen USB transport | complete (EP4 OUT interrupt / EP3 IN interrupt for SMEX) |
| SMEX control message names | complete (13 types, from binary strings) |
| MessageBlock wire format | complete (decoded from serato_device_akaisdk.dll) |
| Service ID routing byte | complete (0x00-0x04 per service) |
| Live communication | not yet verified end-to-end (dwc2/ffs timing issue under investigation) |
