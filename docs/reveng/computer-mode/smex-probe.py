#!/usr/bin/env python3
"""
PRIME 4 Computer Mode - SMEX protocol probe script.

Protocol is UNIDIRECTIONAL (host to device only):
  - EP4 OUT (0x04, interrupt, 64B) - control messages, MUST be exactly 64B
  - EP2 OUT (0x02, bulk, up to 4096B) - bulk data (images etc.)
  - EP3 IN  (0x83) and EP1 IN (0x81) exist in descriptor but planck NEVER writes
    to them - do not expect any responses

Confirmed via strace of planck-remote-screen:
  - planck submits 32 AIO PREADs on ep2 and ep4 at startup
  - planck never issues io_submit PWRITE or write() to ep1 or ep3
  - 64B padding is REQUIRED for EP4 - short packets cause io_getevents to never fire

Wire format (MessageBlock):
  [uint32 BE: content length]
  [uint8:     service ID]     0x00=SMEX 0x01=PingPong 0x02=Image 0x03=Touch
  [service payload...]

SmexControlService (service 0x00):
  [uint32 BE: type name length]
  [bytes:     type name UTF-8]   e.g. "smex.protocolversion"
  [uint32 BE: value length]
  [bytes:     value UTF-8]

PingPongService (service 0x01):
  [uint32 LE: sequence ID]
  [uint8:     type]            0x01=ping, 0x03=pong
"""
import usb.core, usb.util, struct, time, sys

VENDOR  = 0x15e4
PRODUCT = 0xa008

EP_CTRL_OUT  = 0x04   # interrupt OUT - SMEX control (64B, must pad)
EP_BULK_OUT  = 0x02   # bulk OUT - large data


def pad64(data):
    """Pad message to exactly 64 bytes (required for EP4 interrupt endpoint)."""
    assert len(data) <= 64, f"SMEX message too large ({len(data)}B > 64B)"
    return data + bytes(64 - len(data))


def build_smex(type_str, payload=""):
    """Build a SMEX MessageBlock for EP4 OUT."""
    tb = type_str.encode('utf-8')
    pb = payload.encode('utf-8') if isinstance(payload, str) else payload
    inner = bytes([0x00])
    inner += struct.pack('>I', len(tb)) + tb
    inner += struct.pack('>I', len(pb)) + pb
    return struct.pack('>I', len(inner)) + inner


def main():
    dev = usb.core.find(idVendor=VENDOR, idProduct=PRODUCT)
    if dev is None:
        print("PRIME 4 Screen not found. Is the device in Computer Mode?")
        sys.exit(1)

    print(f"Found PRIME 4 Screen: bus={dev.bus} dev={dev.address}")

    try:
        if dev.is_kernel_driver_active(0):
            dev.detach_kernel_driver(0)
    except: pass

    try:
        dev.set_configuration()
        print("SET_CONFIGURATION sent")
    except Exception as e:
        print(f"SET_CONFIGURATION: {e} (may already be configured)")

    time.sleep(0.2)
    usb.util.claim_interface(dev, 0)
    print("Interface 0 claimed")

    # Send smex.protocolversion (must be padded to 64B)
    print("\n--- Sending smex.protocolversion ---")
    msg = build_smex("smex.protocolversion", "1")
    print(f"  EP4 OUT (padded to 64B): {pad64(msg).hex()}")
    dev.write(EP_CTRL_OUT, pad64(msg), timeout=2000)
    time.sleep(0.1)

    # Send smex.time.request
    print("\n--- Sending smex.time.request ---")
    msg = build_smex("smex.time.request")
    print(f"  EP4 OUT (padded to 64B): {pad64(msg).hex()}")
    dev.write(EP_CTRL_OUT, pad64(msg), timeout=2000)
    time.sleep(0.1)

    # Send smex.brightness.set with value 50
    print("\n--- Sending smex.brightness.set = 50 ---")
    msg = build_smex("smex.brightness.set", "50")
    print(f"  EP4 OUT (padded to 64B): {pad64(msg).hex()}")
    dev.write(EP_CTRL_OUT, pad64(msg), timeout=2000)
    time.sleep(0.5)

    # Restore brightness to 100
    print("\n--- Restoring smex.brightness.set = 100 ---")
    msg = build_smex("smex.brightness.set", "100")
    dev.write(EP_CTRL_OUT, pad64(msg), timeout=2000)

    print("\nNote: no responses expected (protocol is unidirectional)")
    print("Check if screen brightness changed to confirm EP4 messages are received")

    usb.util.release_interface(dev, 0)


if __name__ == "__main__":
    main()
