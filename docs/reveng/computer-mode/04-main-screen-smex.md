# Main Screen Protocol - SMEX / MessageBlock (FunctionFS)

Applies to: **PRIME 4 Screen** (`15e4:a008`)

This is the main 7" touch display of the Prime 4 (800x1280 MIPI-DSI, internal).
The host communicates via the FunctionFS vendor-class interface. The device-side
application `planck-remote-screen` receives data and renders to the local framebuffer.

## Sources

- Strings and symbol names from `/usr/bin/planck-remote-screen` (ARM32 ELF, firmware 4.3.4)
- Reverse engineering of `serato_device_akaisdk.dll` (Serato's Akai SDK, 1.5 MB)
- Live USB testing via SSH to the Prime 4 at `icedream-dp4` with direct kernel inspection

---

## USB Endpoint Map

The FunctionFS gadget is named **smexstream** and exposes four endpoints:

| EP address | Direction | Type | Max pkt | Purpose |
|---|---|---|---|---|
| `0x02` | OUT | Bulk | 512B | Host to device - large data (images etc.) |
| `0x81` | IN | Bulk | 512B | Device to host - large data |
| `0x04` | OUT | Interrupt | 64B | **Host to device - SMEX control messages** |
| `0x83` | IN | Interrupt | 64B | **Device to host - SMEX responses** |

**Critical**: SMEX control messages go on the **interrupt endpoints** (EP4 OUT /
EP3 IN), NOT the bulk endpoints. The bulk endpoints (EP2 OUT / EP1 IN) are for
larger data transfers.

---

## Transport

### Device side (planck-remote-screen)

`planck-remote-screen` opens all five FunctionFS fds on startup:

```
/dev/ffs-smexstream/ep0   (control - USB events)
/dev/ffs-smexstream/ep1   (bulk IN  - device to host)
/dev/ffs-smexstream/ep2   (bulk OUT - host to device)
/dev/ffs-smexstream/ep3   (interrupt IN  - device to host)
/dev/ffs-smexstream/ep4   (interrupt OUT - host to device)
```

It uses Linux AIO (`io_submit`, syscall 246) to submit 32 concurrent async
reads on each OUT endpoint. All 9 IOCBs in a typical batch are PREAD on fd=16
(ep2) or fd=18 (ep4). Responses are written to ep3 and ep1.

### Connection sequence

The USB gadget must be fully activated before communication is possible.
The correct sequence from the host side is:

1. Detect the device via `libusb_open_device_with_vid_pid(ctx, 0x15e4, 0xa008)`
2. Call `libusb_set_configuration(h, 1)` **exactly once** to trigger
   `ffs_func_set_alt` in the kernel, which binds the endpoints and wakes up
   `planck-remote-screen`'s AIO reads
3. Claim interface 0
4. Submit reads on EP3 IN (interrupt) AND EP1 IN (bulk) immediately - the device
   may send an initial announcement before the host sends anything
5. Write `smex.protocolversion` to EP4 OUT (interrupt, max 64 bytes)

**Important**: Avoid calling `set_configuration()` more than once per planck
instance. Each call to `set_configuration()` or `claim_interface()` that
sends a SET_CONFIGURATION or SET_INTERFACE request causes `ffs_func_set_alt`
to run again, which re-initialises the endpoints and resets any pending AIO
reads. This creates a race condition where the host's writes land in the DMA
buffer but the completion notification never fires for `planck-remote-screen`.

### Known issue with repeated re-enumeration

During testing it was observed that the dwc2 USB OTG controller on the RK3288
(ff580000.usb) correctly receives bulk/interrupt OUT data into DMA buffers (verified
by reading `/dev/mem` at the DMA address), and the IRQ counter increases with each
host write. However, `total_data` in the kernel's ep debugfs stays at 0 if the
endpoint binding is disrupted mid-transfer.

Root cause: `planck-remote-screen`'s AIO reads block in `ffs_epfile_io` at
`wait_event_interruptible(epfile->wait, ep = epfile->ep)` if `ffs_func_set_alt`
was not called or was called while the io_submit was in flight. Re-submitting
SET_CONFIGURATION from the host wakes this up, but only if done cleanly once at
startup.

---

## MessageBlock Wire Format

Decoded from `serato_device_akaisdk.dll` (Serato SDK):

```
[uint32 BE: total content length]
[uint8:     service ID]            0x00=SmexControl 0x01=PingPong 0x02=ImageStream 0x03=TouchInput 0x04=MessageBlockService
[bytes:     service payload]
```

For **SmexControlService** (service ID `0x00`):

```
[uint32 BE: type string length]
[bytes:     type string UTF-8]    e.g. "smex.protocolversion"
[uint32 BE: payload length]
[bytes:     payload UTF-8]
```

For **PingPongService** (service ID `0x01`):

```
[uint32 BE: sequence ID]
[uint8:     type]                  0x01=ping, 0x03=pong
```

Service registration (from `serato_device_akaisdk.dll`):

| Byte | Service | Direction |
|---|---|---|
| `0x00` | SmexControlService | Bidirectional control |
| `0x01` | PingPongService | Keepalive (2000ms interval) |
| `0x02` | ImageStreamEncoder | Host to device images |
| `0x03` | TouchInputListener | Device to host touch events |
| `0x04` | MessageBlockService | General messages |

---

## SMEX Control Protocol

### Message types (13 total, from `planck-remote-screen` binary strings)

| Message | Direction | Description |
|---|---|---|
| `smex.protocolversion` | both | Version handshake - first message exchanged |
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

The 13 type strings are stored as a sequential pointer table in `.rodata` at
file offset `0x43baa8` (VA `0x443aa8`) of `planck-remote-screen`.

---

## Image Streaming

`ImageStreamEncoder::ReceiveComponentImage(juce::Image, juce::RectangleList<int>)`
on the device side receives rendered component images and displays them on the
device's own local framebuffer (`/dev/fb0`, `/dev/dri/...`). This is **not** a
USB transfer - the display is rendered locally by `planck-remote-screen` after
receiving content metadata (track info, artwork etc.) via SMEX or other means.

Jog wheel images are sent separately over MIDI SysEx (see `03-jog-wheel-sysex.md`).

---

## Library and Deck Display Capability

The `serato_device_akaisdk.dll` exposes the following in `Akai::RemoteScreen::DjDisplay`,
confirming the screen can display rich content from the DJ software:

| Type | Purpose |
|---|---|
| `DeckDisplay` | Deck state (track info, waveform, loop, etc.) |
| `ListRow` | A single row in a library list |
| `BrowserMode` | Current browse mode (library, cue list, etc.) |
| `CuePoint` | Cue point display |
| `WaveformLocation` | Playhead position for waveform |
| `ScrollBar` | List scrollbar state |
| `DeckPressEvent` | Touch/jog input from device |
| `Question` | Dialog/confirmation |
| `DeckDrawingStyle` | Visual deck style |

Host entry point: `CreateAkaiSDK()` returns an opaque object; all calls are
through virtual methods. `AkaiSDKDevices::DeviceConnected` fires when a
device is found.

---

## Direct Device Inspection Results (live testing)

The following was confirmed by SSHing into the Prime 4 at runtime:

- `planck-remote-screen` (PID varies) runs with 3 threads
- All five FunctionFS fds (ep0-ep4) are open
- `boost::asio` io_context submits AIO reads in batches of 9 (PREAD, fd=16/ep2 or fd=18/ep4)
- The UDC (`ff580000.usb`, RK3288 dwc2) correctly writes host data into DMA buffers
  (verified by reading `/dev/mem` at the DOEPDMA address)
- IRQ count increases by 1 per host write, confirming hardware receipt
- The `wait_event_interruptible(epfile->wait, ep = epfile->ep)` in `ffs_epfile_io`
  can block the AIO submission if `ffs_func_set_alt` has not completed
- A soft-disconnect/reconnect cycle (`echo disconnect/connect > soft_connect`) followed
  by one clean SET_CONFIGURATION from the host resolves the blockage

### Minimal working connection sequence (pseudocode)

```python
dev = libusb.open(vid=0x15e4, pid=0xa008)
dev.set_configuration(1)          # triggers ffs_func_set_alt once
dev.claim_interface(0)

# Keep IN endpoints polled continuously (device may initiate first)
start_async_reader(EP3_IN=0x83, max=64)
start_async_reader(EP1_IN=0x81, max=512)

# Send version handshake on interrupt OUT
smex_version = build_smex("smex.protocolversion", "1")
dev.write(EP4_OUT=0x04, smex_version)

# Wait for smex.version response on EP3 IN
response = read(EP3_IN=0x83, timeout=3000)
```
