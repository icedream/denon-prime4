# Main Screen Protocol - SMEX / MessageBlock (FunctionFS Bulk)

Applies to: **PRIME 4 Screen** (`15e4:a008`)

This is the main 7" touch display of the Prime 4 (800x1280 MIPI-DSI, internal).
The host communicates with it via **bulk USB transfers** over the FunctionFS
vendor-class interface. The device-side application `planck-remote-screen`
receives data and renders to the local framebuffer.

## Source

All findings in this document come from the JP11 firmware rootfs, specifically
from strings and symbol names extracted from `/usr/bin/planck-remote-screen`
(ARM32 ELF, 11 MB).

---

## Transport

### Host side

Open the vendor-class interface with libusb (no kernel driver needed - the
interface is unbound on Linux by default):

```c
libusb_device_handle *h = libusb_open_device_with_vid_pid(ctx, 0x15e4, 0xa008);
libusb_claim_interface(h, 0);
libusb_bulk_transfer(h, 0x02 /* EP2 OUT */, buf, len, &transferred, timeout_ms);
libusb_bulk_transfer(h, 0x81 /* EP1 IN  */, buf, len, &transferred, timeout_ms);
```

### Device side

`planck-remote-screen` reads/writes via FunctionFS file descriptors:

```
/dev/ffs-{gadget_name}/ep1   (IN  bulk)
/dev/ffs-{gadget_name}/ep2   (OUT bulk)
/dev/ffs-{gadget_name}/ep3   (IN  interrupt)
/dev/ffs-{gadget_name}/ep4   (OUT interrupt)
```

Uses `boost::asio` (io_context) for async I/O with 32 outstanding requests
per endpoint (`OutEndpoint<FsOutRequest, 32>`).

---

## MessageBlock Framing

The framing layer is `MessageBlockStream`. The same format is used over both
USB (via `UsbGadgetMessageBlockStream`) and TCP (via `NetworkMessageBlockStream`).

Key classes found in the binary:

| Class | Role |
|---|---|
| `MessageBlockBuilderOutputStream` | Serialises a message into wire bytes |
| `MessageBlockReader` | Deserialises bytes into a message |
| `MessageBlockRouter` | Routes messages to handlers by type |
| `MessageBlockService` | Service endpoint for receiving messages |
| `ConnectableMessageBlockStream` | State-machine wrapper (connecting/connected/disconnected) |
| `UsbGadgetMessageBlockStream` | Bulk-USB transport |
| `NetworkMessageBlockStream` | TCP transport (same wire format) |
| `FsTransferMessageBlockStream` | Filesystem transport (firmware update images) |

Wire format: not yet fully decoded. Further analysis of `planck-remote-screen`
required.

---

## SMEX Control Protocol

The control channel uses named string message types (`smex.*`).
Implemented by `SmexControlService` on the device side.
The client namespace is `Acvt` (`Acvt::SmexClient`).

The name "SMEX" appears to be an Akai-derived protocol - the binary contains
`Akai::RemoteScreen` as a C++ namespace.

### Known message types (from binary strings)

| Message | Direction | Description |
|---|---|---|
| `smex.protocolversion` | both | Version handshake - sent first by both sides |
| `smex.version` | device to host | Device firmware version string |
| `smex.time.request` | host to device | Request device clock |
| `smex.time.set` | host to device | Set device clock |
| `smex.brightness.request` | host to device | Query current brightness |
| `smex.brightness.set` | host to device | Set screen brightness |
| `smex.screensaver.set` | host to device | Enable/disable screensaver |
| `smex.battery` | device to host | Battery level (battery-powered devices only) |
| `smex.powerkey` | device to host | Power button pressed |
| `smex.restart.powerdown` | host to device | Power off the device |
| `smex.restart.standalone` | host to device | Return to standalone Engine mode |
| `smex.restart.updater` | host to device | Boot into firmware updater |
| `smex.unknown` | - | Handler for unrecognised type |

### Connection

The `PingPongListener::Connection` type indicates a ping/pong keepalive
mechanism on the connection.

---

## Image Streaming

The `ImageStreamEncoder::ReceiveComponentImage(juce::Image, juce::RectangleList<int>)`
method on the device side receives rendered component images and displays them on the
device's own 800x1280 MIPI-DSI screen via the local DRM/KMS framebuffer (`/dev/fb0`,
`/dev/dri/...`). This is **not** a USB transfer - the display is rendered locally.

Jog wheel images are sent separately over the MIDI SysEx interface (see `03-jog-wheel-sysex.md`).

The `UsbGadgetMessageBlockStream` (FunctionFS bulk stream) carries only **small SMEX
control messages** - not pixel or image data.

---

## Key Binary Symbols (planck-remote-screen)

C++ class names from RTTI strings in the binary:

```
RemoteScreenApp
RemoteScreenMainWindow
RemoteScreenConnectedComponent
RemoteScreenNotConnectedComponent
RemoteScreenService
ImageStreamEncoder
UsbGadgetMessageBlockStream
NetworkMessageBlockStream
FsTransferMessageBlockStream
MessageBlockRouter
MessageBlockBuilderOutputStream
SmexControlHost
SmexControlClient
SmexControlService
Acvt::SmexClient
UsbGadgetVolumeControl
MidiOutputServiceClient
ScreensaverTimer
ScreensaverComponent
```
