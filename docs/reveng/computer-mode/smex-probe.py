#!/usr/bin/env python3
"""
PRIME 4 Computer Mode - SMEX protocol probe script.

Run this after the device is switched to Computer Mode
(hold VIEW -> Sources -> laptop+USB icon -> Yes).

MessageBlock wire format (confirmed from serato_device_akaisdk.dll):
  [uint32 BE: total content length]
  [uint32 BE: type string length]
  [bytes: type string UTF-8]
  [uint32 BE: payload length]
  [bytes: payload]
"""
import usb.core, usb.util, struct, time, os, sys

VENDOR  = 0x15e4
PRODUCT = 0xa008  # PRIME 4 Screen
EP_OUT  = 0x02    # EP2 OUT bulk - host -> device
EP_IN   = 0x81    # EP1 IN bulk  - device -> host
EP_INT_OUT = 0x04
EP_INT_IN  = 0x83


def open_device():
    dev = usb.core.find(idVendor=VENDOR, idProduct=PRODUCT)
    if dev is None:
        print("PRIME 4 Screen not found. Is the device in Computer Mode?")
        sys.exit(1)
    if dev.is_kernel_driver_active(0):
        dev.detach_kernel_driver(0)
    usb.util.claim_interface(dev, 0)
    print(f"Opened PRIME 4 Screen (bus {dev.bus}, dev {dev.address})")
    return dev


def read_all(dev, timeout=1000):
    results = []
    for ep, sz, name in [(EP_IN, 512, "bulk"), (EP_INT_IN, 64, "intr")]:
        try:
            d = dev.read(ep, sz, timeout=timeout)
            results.append((name, bytes(d)))
        except usb.core.USBTimeoutError:
            pass
        except Exception as e:
            results.append((name, f"ERR:{e}"))
    return results


def parse_response(raw):
    """Try to parse an incoming MessageBlock."""
    if len(raw) < 4:
        return None
    total_len = struct.unpack_from('>I', raw, 0)[0]
    if len(raw) < 4 + total_len:
        return None
    pos = 4
    strings = []
    while pos < 4 + total_len:
        if pos + 4 > len(raw):
            break
        slen = struct.unpack_from('>I', raw, pos)[0]
        pos += 4
        if pos + slen > len(raw):
            break
        s = raw[pos:pos+slen]
        strings.append(s)
        pos += slen
    return strings


def messageblock(type_str, payload=b""):
    """Build a MessageBlock with the confirmed wire format."""
    type_b    = type_str.encode('utf-8')
    inner     = (struct.pack('>I', len(type_b)) + type_b +
                 struct.pack('>I', len(payload)) + payload)
    return struct.pack('>I', len(inner)) + inner


def probe(dev, label, payload, ep=EP_OUT, timeout=2000):
    try:
        n = dev.write(ep, payload, timeout=1000)
        time.sleep(0.15)
        resp = read_all(dev, timeout)
        if resp:
            for ch, d in resp:
                if isinstance(d, bytes):
                    parsed = parse_response(d)
                    print(f"  [{label}] RESPONSE on {ch} ({len(d)}B): {d.hex()}")
                    if parsed:
                        for i, s in enumerate(parsed):
                            try:
                                print(f"    string[{i}]: {repr(s.decode('utf-8'))}")
                            except:
                                print(f"    string[{i}]: {s.hex()}")
                    return d
        else:
            print(f"  [{label}] no response ({n}B sent)")
    except Exception as e:
        print(f"  [{label}] error: {e}")
    return None


def main():
    dev = open_device()

    print("\n--- Listening for spontaneous data (3s) ---")
    for ch, d in read_all(dev, 3000):
        if isinstance(d, bytes):
            print(f"  Spontaneous {ch}: {d.hex()}")
            parsed = parse_response(d)
            if parsed:
                for i, s in enumerate(parsed):
                    print(f"    string[{i}]: {repr(s.decode('utf-8', errors='replace'))}")

    print("\n--- CONFIRMED FORMAT: [total_len][type_len][type][payload_len][payload] ---")

    # Protocol version handshake - the first message to send
    print("\n[1] smex.protocolversion (version=1)")
    msg = messageblock("smex.protocolversion", b"\x00\x00\x00\x01")
    probe(dev, "protocolversion", msg)

    print("\n[2] smex.version (empty payload)")
    probe(dev, "version", messageblock("smex.version"))

    print("\n[3] smex.time.request (empty)")
    probe(dev, "time.request", messageblock("smex.time.request"))

    print("\n[4] smex.brightness.request (empty)")
    probe(dev, "brightness.request", messageblock("smex.brightness.request"))

    # Try sending version=1 as just the string "1"
    print("\n[5] smex.protocolversion payload='1'")
    probe(dev, "protocolversion-str", messageblock("smex.protocolversion", b"1"))

    # Library browsing - the DjDisplay structs include ListRow, BrowserMode
    print("\n[6] Testing library commands via smex")
    for t in ["smex.unknown", "screen.library", "dj.library.request"]:
        probe(dev, t, messageblock(t))

    # Try sending on interrupt endpoint instead
    print("\n--- Interrupt endpoint ---")
    for t in ["smex.protocolversion", "smex.version"]:
        msg = messageblock(t, b"\x00\x00\x00\x01")
        probe(dev, f"intr:{t}", msg, ep=EP_INT_OUT, timeout=1000)

    usb.util.release_interface(dev, 0)
    print("\nDone. Run with device in Computer Mode for best results.")


if __name__ == "__main__":
    main()
