# USB Device Layout

When the Prime 4 is in Computer Mode, it exposes **5 separate USB devices**
through a USB gadget named `smexstream`, configured by `planck-remote-screen`
using `libusbgx` and FunctionFS.

## Vendor / Product IDs

All devices use **VID `0x15e4`** (Numark, InMusic Brands).

| Product ID | Name | USB Class | Linux driver |
|---|---|---|---|
| `0x8008` | PRIME 4 Control Surface | USB Audio / MIDI Streaming | `snd-usb-audio` |
| `0x9008` | PRIME 4 Left Wheel Display | USB Audio / MIDI Streaming | `snd-usb-audio` |
| `0xb008` | PRIME 4 Right Wheel Display | USB Audio / MIDI Streaming | `snd-usb-audio` |
| `0xc008` | PRIME 4 Audio | USB Audio 2.0 + MIDI | `snd-usb-audio` |
| **`0xa008`** | **PRIME 4 Screen** | **Vendor Specific (0xFF/0x01)** | **unbound** |

## PRIME 4 Screen (0xa008) - full descriptor

Interface `9-1:1.0` (sysfs name when on bus 9):
- Class: `0xFF` (Vendor Specific)  
- SubClass: `0x01`
- Protocol: `0x00`
- Interface string: `"Denon DJ Remote Screen"`

Endpoints:

| EP | Address | Dir | Type | Max pkt | Purpose |
|---|---|---|---|---|---|
| EP1 | `0x81` | IN | Bulk | 512B | Device → Host data |
| EP2 | `0x02` | OUT | Bulk | 512B | **Host → Device** (main data channel) |
| EP3 | `0x83` | IN | Interrupt | 64B | Device → Host events |
| EP4 | `0x04` | OUT | Interrupt | 64B | Host → Device commands |

USB negotiated speed: **High Speed (480 Mbps)**

## USB Gadget Configuration

The gadget is configured via `/sys/kernel/config/usb_gadget/smexstream/` using
`libusbgx`. Known configfs paths written by `planck-remote-screen`:

```
/sys/kernel/config/usb_gadget/smexstream/functions/midi.midi
/sys/kernel/config/usb_gadget/smexstream/functions/uac2.audio
/sys/kernel/config/usb_gadget/smexstream/functions/uac2.audio/c_hs_bint
/sys/kernel/config/usb_gadget/smexstream/functions/uac2.audio/explicit_feedback
/sys/kernel/config/usb_gadget/smexstream/functions/uac2.audio/fb_max
/sys/kernel/config/usb_gadget/smexstream/functions/uac2.audio/named_channels
/sys/kernel/config/usb_gadget/smexstream/functions/uac2.audio/p_hs_bint
/sys/kernel/config/usb_gadget/smexstream/functions/uac2.audio/req_number
/sys/kernel/config/usb_gadget/smexstream/functions/uac2.audio/shared_clock
```

The Screen interface (`0xa008`) is implemented via **FunctionFS** (`/dev/ffs-*`),
allowing `planck-remote-screen` to handle it in user-space. The FunctionFS
endpoints are exposed at `/dev/ffs-{name}/ep1` through `/ep4`.

The vendor and product IDs are set at runtime via:
- `usbg_set_gadget_vendor_id(0x15e4)`
- `usbg_set_gadget_product_id(0xa008)` (and per-function variants)

## ALSA MIDI Ports (Linux host)

After the USB MIDI devices enumerate, the following ALSA ports appear:

| Card | Name |
|---|---|
| card 3 | PRIME 4 Right Wheel Display |
| card 4 | PRIME 4 Left Wheel Display |
| card 5 | PRIME 4 Control Surface |
| card 6 | PRIME 4 Audio |

Device nodes: `/dev/snd/midiC3D0` through `/dev/snd/midiC6D0`

On the **device itself**, `planck-remote-screen` accesses the MIDI function at:
```
/dev/snd/by-path/platform-ff540000.usb-usb-0:1:1.0
/dev/snd/by-path/platform-ff540000.usb-usb-0:1.2:1.0
```
