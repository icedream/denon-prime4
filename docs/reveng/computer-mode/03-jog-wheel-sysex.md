# Jog Wheel Display Protocol (SysEx over MIDI)

Applies to: **PRIME 4 Left Wheel Display** (`15e4:9008`) and  
**PRIME 4 Right Wheel Display** (`15e4:b008`).

Source: `JC11_Display_Device.qml` and `JC11_Display_Assignments.qml`  
in `/usr/Engine/AssignmentFiles/PresetAssignmentFiles/JC11/`

The JC11 is the jog wheel controller module used inside the Prime 4.
All display communication goes over USB MIDI (ALSA `snd-usb-audio`).

---

## SysEx Format

All jog display commands use the Denon manufacturer SysEx header:

```
F0 00 02 0B [device_id] 08 [command] 00 [payload_len] [payload...] F7
```

- `F0` - SysEx start
- `00 02 0B` - Denon DJ manufacturer ID
- `[device_id]` - `0x10` (left) or `0x30` (right); determined by inquiry
- `08` - "display" section byte
- `[command]` - command byte (see table below)
- `00 [payload_len]` - 2-byte payload length (big-endian)
- `[payload...]` - command-specific data
- `F7` - SysEx end

### Device ID Discovery

Send MIDI Universal Identity Request:
```
F0 7E 00 06 01 F7
```
Device responds with inquiry reply; **byte index 15** of the response contains
the device ID (`0x10` = left, `0x30` = right).

---

## Command Reference

### `0x10` - Start Image Decode

```
F0 00 02 0B [id] 08 10 00 00 F7
```

Sent on init. The device responds with a SysEx message containing `code=0x10`
when it is ready to receive image data (`imageSendingEnabled = true`).

Also used with a **17-second timeout**: if no response within 17s, the host
assumes the device is ready anyway.

### `0x7C` - Set Brightness

```
F0 00 02 0B [id] 08 7C 00 01 [level] F7
```

| Level | Value |
|---|---|
| Off (dark) | `0x00` |
| Low | `0x0F` (15) |
| Mid | `0x46` (70) |
| High | `0x69` (105) |
| Max | `0x7F` (127) |

### `0x0B` - Set Element Color (ARGB)

```
F0 00 02 0B [id] 08 0B 00 09 [elem_id] [A_hi] [A_lo] [R_hi] [R_lo] [G_hi] [G_lo] [B_hi] [B_lo] F7
```

Each color component is encoded as **two nibbles** (7-bit each):
```javascript
function colorComponentToHex(component) {
    var ci = Math.floor(component * 255)
    return d2h((ci & 0xF0) >> 4) + " " + d2h(ci & 0x0F)
}
```

#### Element IDs

| ID | Element |
|---|---|
| 0 | Album Artwork (alpha only, `Qt.rgba(1,1,1,alpha)`) |
| 1 | Engine Logo |
| 2 | Platter Position Ring |
| 3 | Platter Position Indicator |
| 4 | Slip Position Ring (alpha, `Qt.rgba(0,0,0,alpha)`) |
| 5 | Slip Position Indicator |
| 6 | Track Progress Ring |
| 7 | Track Progress Indicator |
| 8 | Loop and Layer Text |
| 9 | Burst Image |
| 10 | Jump Icon |
| 11 | Loop Icon |

### `0x0A` - Update Display Elements

```
F0 00 02 0B [id] 08 0A 00 04 [byte0] [byte1] [artwork_idx] [text_idx] F7
```

#### byte0 - Element Enable/Disable 1

| Bit | Element |
|---|---|
| 0 | Album Artwork |
| 1 | Engine Logo |
| 2 | Platter Position Ring |
| 4 | Slip Position Ring (active) |
| 5 | Slip Position Indicator (active) |

#### byte1 - Element Enable/Disable 2

| Bit | Element |
|---|---|
| 1 | Loop and Layer Text |
| 3 | Jump Icon |
| 4 | Loop Icon |

#### artwork_idx - Album Artwork Index
- `0` = no artwork / placeholder
- `1` = Deck 1 artwork
- `2` = Deck 3 artwork (second layer)
- `3` = Device artwork (CurrentDeviceArtwork)

#### text_idx - Loop / Layer Text Code

| Code | Text |
|---|---|
| 0x0 | "1/64" |
| 0x1 | "1/32" |
| 0x2 | "1/16" |
| 0x3 | "1/8" |
| 0x4 | "1/4" |
| 0x5 | "1/2" |
| 0x6 | "1" |
| 0x7 | "2" |
| 0x8 | "4" |
| 0x9 | "8" |
| 0xA | "16" |
| 0xB | "32" |
| 0xC | "64" |
| 0xD | "--" (default/unknown) |
| 0xE | "A" |
| 0xF | "B" |
| 0x10 | "3" |
| 0x11 | "6" |
| 0x12 | "9" |
| 0x13 | "C" |
| 0x14 | "D" |

---

## Image Sending

Album artwork and other images are rendered offscreen by the host, encoded as
**PNG**, and sent as a sequence of SysEx packets.

### Render Pipeline

1. **Painter** QML object with `format: Painter.AZ01_CENTER_WHEEL_DISPLAY`
2. Canvas size: **198 × 198 pixels**
3. Draw steps:
   ```qml
   begin()
   clear("white")
   translate(width, height)
   rotate(180)
   drawByteArray(albumArt, 0, 0, width, height)
   drawHole("black", Qt.rect(0,0,w,h), Qt.point(w/2, h/2), (w/2)-4)
   ```
   - The center hole mimics the jog wheel platter hole.
4. Convert to SysEx:
   ```qml
   var data = imageToByteArrays(artworkIndex, 0x08, device.midiDeviceId)
   Midi.sendByteArrays(data)
   ```

`imageToByteArrays(index, section, deviceId)` - host-side QML API:
- Encodes the rendered PNG into Denon SysEx chunks
- `index` = artwork slot (1 or 2 for left/right deck, 3 for device artwork)
- `section` = `0x08` (display section)
- Returns an array of `QByteArray` objects, each one a complete SysEx message

### Device Response

After the host sends image data, the device replies with a SysEx containing:
- `code = 0x10` → decoding complete, ready for display updates
- `code = 0x0f` → upload status; `byte[9] = 0x00` = success, non-zero = error

---

## Platter Position (Realtime)

Sent via **MIDI Pitch Bend** messages (not SysEx) at 12ms intervals:

```qml
Timer { interval: 12; repeat: true
    onTriggered: {
        Midi.sendPitch(0, angle)       // main platter angle
        Midi.sendPitch(1, slipAngle)   // slip mode angle
    }
}
```

Position source: `/Private/Deck%d/MidiSamplePosition`

Angle calculation:
```javascript
const divider = 60.0 / (33.0 + 1.0/3.0)   // seconds per revolution at 33⅓ RPM
const mod = divider * 44100.0               // samples per revolution
const modulo = (pos % mod + mod) % mod
const angle = (modulo / 44100.0) / divider  // 0.0 – 1.0
```

---

## Additional SysEx (from JP11_Controller_Device.qml)

These are sent to the **Control Surface** MIDI device, not the wheel displays.

### Query absolute control positions

```
F0 00 02 0B 7F 0C 04 00 00 F7
```

### Initialization / query knob+fader positions

```
F0 00 02 0B 7F 0C 60 00 04 04 01 01 04 F7
```

### Query power-on button state

```
F0 00 02 0B 7F 0C 42 00 00 F7
```

### Set RGB LED color (control surface buttons)

```
F0 00 02 0B 7F 0C 03 00 05 [channel] [index] [R7] [G7] [B7] F7
```
- `[channel]` and `[index]` identify the button
- `[R7] [G7] [B7]` are 7-bit color values (0–127)
