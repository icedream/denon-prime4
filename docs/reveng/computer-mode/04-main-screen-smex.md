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

| EP address | Direction | Type | Max pkt (FS/HS) | Used by planck | Purpose |
|---|---|---|---|---|---|
| `0x02` | OUT | Bulk | 64B / 512B | Yes (AIO PREAD) | Host to device - large data (images etc.) |
| `0x81` | IN | Bulk | 64B / 512B | **No** | Reserved - planck never writes here |
| `0x04` | OUT | Interrupt | 64B | Yes (AIO PREAD) | **Host to device - SMEX control messages** |
| `0x83` | IN | Interrupt | 64B | **No** | Reserved - planck never writes here |

**Protocol is UNIDIRECTIONAL** (host to device only). Confirmed by strace analysis
of planck's full syscall trace at startup: planck only ever submits `IOCB_CMD_PREAD`
operations on ep2 (fd=16) and ep4 (fd=18). No `IOCB_CMD_PWRITE` or `write()` calls
are ever made to ep1 (fd=15) or ep3 (fd=17) under any circumstances.

**Critical**: SMEX control messages go on the **interrupt endpoints** (EP4 OUT /
EP3 IN), NOT the bulk endpoints. The bulk endpoints (EP2 OUT / EP1 IN) are for
larger data transfers.

---

## Transport

### Device side (planck-remote-screen)

`planck-remote-screen` opens all five FunctionFS fds on startup:

```
/dev/ffs-smexstream/ep0   (control - USB events, fd=14)
/dev/ffs-smexstream/ep1   (bulk IN  - device to host, fd=15) <- UNUSED
/dev/ffs-smexstream/ep2   (bulk OUT - host to device, fd=16) <- reads via AIO
/dev/ffs-smexstream/ep3   (interrupt IN  - device to host, fd=17) <- UNUSED
/dev/ffs-smexstream/ep4   (interrupt OUT - host to device, fd=18) <- reads via AIO
```

Startup sequence (verified via strace with -f flag):
1. Writes USB descriptor to ep0 (`write(14, descriptor, 133)`) - two alternate settings
2. Writes UDC name to fd=20 (`write(20, "ff580000.usb", 12)`) - binds gadget
3. Prints "Gadget state changed: connected" once host sends SET_CONFIGURATION
4. Submits **32 AIO PREADs** on ep2 (fd=16), each 4096 bytes
5. Submits **32 AIO PREADs** on ep4 (fd=18), each 64 bytes

On each AIO completion (host wrote data):
- `io_getevents` harvests the result (`res=64` for ep4, `res=4096` for ep2)
- One new PREAD is re-submitted to maintain the pool
- The FunctionFS thread calls the AIO callback inline (no JUCE notification)
- **No writes to ep3 or ep1 occur at any point**

### Connection sequence

The USB gadget must be fully activated before communication is possible.
The correct sequence from the host side is:

1. Detect the device via `libusb_open_device_with_vid_pid(ctx, 0x15e4, 0xa008)`
2. Call `libusb_set_configuration(h, 1)` to trigger `ffs_func_set_alt` in the
   kernel, which binds the endpoints and starts `planck-remote-screen`'s AIO reads
3. Claim interface 0
4. Send SMEX commands to EP4 OUT (interrupt, **must be exactly 64 bytes** padded)
5. Send image/bulk data to EP2 OUT (bulk, up to 4096 bytes per transfer)

**No response is expected** - the protocol is unidirectional. EP3 IN and EP1 IN
exist in the USB descriptor but planck never writes to them.

### 64B padding requirement

All messages sent to EP4 OUT (interrupt endpoint, mps=64) **must be padded to
exactly 64 bytes**. Messages shorter than 64 bytes cause the FunctionFS interrupt
endpoint to not signal completion properly, and `io_getevents` on the device side
never returns for those transfers. This was confirmed experimentally:
- Unpadded messages: `io_getevents` never fires, message is silently dropped
- 64B padded messages: `io_getevents` returns `res=64`, message is processed

For EP2 OUT (bulk), messages can be variable length up to 4096 bytes per transfer.

### Known issue with repeated re-enumeration

During testing it was observed that the dwc2 USB OTG controller on the RK3288
(ff580000.usb) correctly receives OUT data into DMA buffers (verified
by reading `/dev/mem` at the DOEPDMA address), and the IRQ counter increases with
each host write. However, `total_data` in the kernel's ep debugfs stays at 0 if
messages are not exactly 64 bytes for EP4.

The 64B padding requirement explains the earlier "kernel bug" hypothesis - the
transfers were not completing because messages were short-packets.

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

### Startup sequence (confirmed via strace)

When the host sends SET_CONFIGURATION:
1. planck reads 12-byte FUNCTIONFS_ENABLE event from ep0
2. Submits 32 AIO PREADs on ep2 (4096B each) and 32 on ep4 (64B each)
3. Previous reads complete with `res=-108` (ESHUTDOWN)
4. Resubmits fresh 64 reads; new messages now complete with `res=64`
5. Prints "Gadget state changed: connected"

AIO callback object structure per read:
```
[4B magic "PGD\0"][4B vtable (JUCE lib)][4B self-ptr]...
[4B ID][4B buf_ptr][4B buf_size=64]
```

### State machine blocker

SMEX commands ARE received (confirmed by strace `io_getevents res=64` with correct
data in buffer). However NO SMEX handler produces any visible effect.

`Acvt::SmexClient` requires three conditions before dispatching handlers:
1. `ConnectableMessageBlockStream::State = CONNECTED` (set by ENABLE event)
2. `PingPongListener::Connection = CONNECTED` (**BLOCKER**)
3. A `bool` enable flag

planck **never** submits `IOCB_CMD_PWRITE` to ep3 under any circumstances (exhaustively
verified by strace across all 3 threads). The PingPong pong cannot be sent over USB,
so `SmexClient.connected` never becomes `true`, gating all SMEX handlers.

Tried without success: smex.protocolversion versions 1-4, PingPong ping/pong with
various seq IDs, EP2 bulk sends, 3+ second delays between messages.

### Known working connection sequence (pseudocode - INCOMPLETE)

```python
dev = libusb.open(vid=0x15e4, pid=0xa008)
dev.set_configuration(1)          # triggers ENABLE event on ep0
dev.claim_interface(0)

# TODO: unknown step to satisfy PingPongListener.connected
# Without this, all SMEX handlers remain gated by SmexClient state machine

# These ARE received by planck but currently have no effect:
dev.write(0x04, pad64(build_smex("smex.protocolversion", "1")))
dev.write(0x04, pad64(build_smex("smex.brightness.set", "80")))
```

Note: EP3 IN (0x83) and EP1 IN (0x81) exist in the USB descriptor but planck
never writes to them. Do not expect responses.
