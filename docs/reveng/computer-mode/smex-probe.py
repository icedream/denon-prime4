#!/usr/bin/env python3
"""
PRIME 4 Computer Mode - SMEX protocol probe script.

Run this after the device is switched to Computer Mode
(hold VIEW -> Sources -> laptop+USB icon -> Yes).

The script tries multiple candidate MessageBlock wire formats
to discover the correct framing for SMEX control messages.
"""
import usb.core, usb.util, struct, time, os, sys

VENDOR  = 0x15e4
PRODUCT = 0xa008  # PRIME 4 Screen
EP_OUT  = 0x02    # EP2 OUT bulk - host -> device
EP_IN   = 0x81    # EP1 IN bulk  - device -> host
EP_INT_OUT = 0x04 # EP4 OUT interrupt
EP_INT_IN  = 0x83 # EP3 IN interrupt

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
    """Try to read from both IN endpoints."""
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

def probe(dev, label, payload, timeout=2000):
    try:
        n = dev.write(EP_OUT, payload, timeout=1000)
        time.sleep(0.1)
        resp = read_all(dev, timeout)
        if resp:
            for ch, d in resp:
                if isinstance(d, bytes):
                    print(f"  [{label}] RESPONSE on {ch}: {d.hex()}")
                    return d
        else:
            print(f"  [{label}] no response ({n}B sent)")
    except Exception as e:
        print(f"  [{label}] error: {e}")
    return None

def smex_msg_utf8(type_str, payload_bytes=b""):
    """
    Candidate format 1: [4B BE total-len][4B BE type-str-len][type-str UTF-8][4B BE payload-len][payload]
    """
    type_b = type_str.encode('utf-8')
    inner = struct.pack('>I', len(type_b)) + type_b + struct.pack('>I', len(payload_bytes)) + payload_bytes
    return struct.pack('>I', len(inner)) + inner

def smex_msg_cstr(type_str, payload_bytes=b""):
    """
    Candidate format 2: [4B BE total-len][null-terminated type-str][4B BE payload-len][payload]
    """
    type_b = type_str.encode('utf-8') + b'\x00'
    inner = type_b + struct.pack('>I', len(payload_bytes)) + payload_bytes
    return struct.pack('>I', len(inner)) + inner

def smex_msg_id(type_id, payload_bytes=b""):
    """
    Candidate format 3: [4B BE total-len][4B BE type-id][payload]
    (numeric type IDs, no string)
    """
    inner = struct.pack('>I', type_id) + payload_bytes
    return struct.pack('>I', len(inner)) + inner

def stagelinq_msg(msg_id, payload):
    """
    Candidate format 4: StageLinQ wire format [4B BE len][payload]
    where payload = [4B BE msgID][content...]
    """
    inner = struct.pack('>I', msg_id) + payload
    return struct.pack('>I', len(inner)) + inner

def main():
    dev = open_device()

    print("\n--- Listening for spontaneous data (5s) ---")
    for ch, d in read_all(dev, 5000):
        if isinstance(d, bytes):
            print(f"  Spontaneous {ch}: {d.hex()}")

    token = os.urandom(16)

    print("\n--- Format 1: [len][utf8-type-len][utf8-type][payload-len][payload] ---")
    for t in ["smex.protocolversion", "smex.version"]:
        probe(dev, f"fmt1:{t}", smex_msg_utf8(t, b"\x00\x00\x00\x01"))

    print("\n--- Format 2: [len][cstr-type][payload-len][payload] ---")
    for t in ["smex.protocolversion", "smex.version"]:
        probe(dev, f"fmt2:{t}", smex_msg_cstr(t, b"\x00\x00\x00\x01"))

    print("\n--- Format 3: [len][type-id uint32][payload] (numeric IDs 0..5) ---")
    for i in range(6):
        probe(dev, f"fmt3:id={i}", smex_msg_id(i, b"\x01"))

    print("\n--- Format 4: StageLinQ service announcement ---")
    def utf16(s): b = s.encode('utf-16-be'); return struct.pack('>I',len(b))+b
    svc = token + utf16("Mixxx") + struct.pack('>H', 51337)
    probe(dev, "stagelinq:service-announce", stagelinq_msg(0x00000000, svc))

    print("\n--- Format 4b: StageLinQ services request ---")
    probe(dev, "stagelinq:services-request", stagelinq_msg(0x00000002, token))

    print("\n--- Format 5: plain [len][payload] with 'smex' magic ---")
    for magic in [b"smex", b"SMEX", b"\x00\x00\x00\x01"]:
        probe(dev, f"magic:{magic.hex()}", struct.pack('>I', 8) + magic + b"\x00"*4)

    print("\n--- Format 6: interrupt OUT endpoint ---")
    for t in ["smex.protocolversion", "smex.version"]:
        try:
            n = dev.write(EP_INT_OUT, smex_msg_utf8(t, b""), timeout=500)
            time.sleep(0.1)
            resp = read_all(dev, 1000)
            if resp:
                print(f"  [intr:{t}] RESPONSE: {resp}")
            else:
                print(f"  [intr:{t}] no response ({n}B sent)")
        except Exception as e:
            print(f"  [intr:{t}] error: {e}")

    usb.util.release_interface(dev, 0)
    print("\nDone.")

if __name__ == "__main__":
    main()
