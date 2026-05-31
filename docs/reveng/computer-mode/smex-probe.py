#!/usr/bin/env python3
"""
PRIME 4 Computer Mode - SMEX protocol probe script.

Confirmed wire format (from serato_device_akaisdk.dll reverse engineering):

  MessageBlock:
    [uint32 BE: total length]
    [uint8:     service ID]    0x00=SMEX 0x01=PingPong 0x02=Image 0x03=Touch 0x04=MsgBlock
    [service payload...]

  SmexControlService payload (service 0x00):
    [uint32 BE: type name length]
    [bytes:     type name UTF-8]
    [uint32 BE: payload length]
    [bytes:     payload UTF-8]

  PingPongService payload (service 0x01):
    [uint32 BE: sequence ID]
    [uint8:     type]          0x01=ping, 0x03=pong

Endpoint map:
  EP4 OUT (0x04, interrupt, 64B max) - host to device SMEX control
  EP3 IN  (0x83, interrupt, 64B max) - device to host SMEX responses
  EP2 OUT (0x02, bulk, 512B)         - host to device bulk data
  EP1 IN  (0x81, bulk, 512B)         - device to host bulk data

Connection sequence:
  1. set_configuration(1)  - triggers ffs_func_set_alt ONCE
  2. claim_interface(0)
  3. Start reading EP3 IN and EP1 IN continuously
  4. Write smex.protocolversion to EP4 OUT
"""
import usb.core, usb.util, struct, time, threading, sys

VENDOR  = 0x15e4
PRODUCT = 0xa008

EP_SMEX_OUT  = 0x04   # interrupt OUT - SMEX control messages
EP_SMEX_IN   = 0x83   # interrupt IN  - SMEX responses
EP_BULK_OUT  = 0x02   # bulk OUT - large data
EP_BULK_IN   = 0x81   # bulk IN  - large data


def build_smex(type_str, payload=""):
    """Build a SMEX MessageBlock for EP4 OUT (interrupt, max 64B)."""
    tb = type_str.encode('utf-8')
    pb = payload.encode('utf-8') if isinstance(payload, str) else payload
    inner = bytes([0x00])  # service ID = SmexControlService
    inner += struct.pack('>I', len(tb)) + tb
    inner += struct.pack('>I', len(pb)) + pb
    return struct.pack('>I', len(inner)) + inner


def build_ping(seq=1):
    """Build a PingPong ping message for EP4 OUT."""
    inner = bytes([0x01])  # service ID = PingPongService
    inner += struct.pack('>I', seq) + bytes([0x01])  # seq + ping type
    return struct.pack('>I', len(inner)) + inner


def parse_response(data):
    """Parse an incoming MessageBlock from EP3 IN."""
    if len(data) < 5:
        return None
    total_len = struct.unpack_from('>I', data, 0)[0]
    if len(data) < 4 + total_len:
        return None
    service_id = data[4]
    payload = data[5:4 + total_len]

    if service_id == 0x00:  # SmexControlService
        if len(payload) < 4:
            return {'service': 'smex', 'type': '?', 'value': '?'}
        tlen = struct.unpack_from('>I', payload, 0)[0]
        type_str = payload[4:4+tlen].decode('utf-8', errors='replace')
        vlen = struct.unpack_from('>I', payload, 4+tlen)[0] if len(payload) >= 8+tlen else 0
        value = payload[8+tlen:8+tlen+vlen].decode('utf-8', errors='replace')
        return {'service': 'smex', 'type': type_str, 'value': value}
    elif service_id == 0x01:  # PingPongService
        if len(payload) >= 5:
            seq = struct.unpack_from('>I', payload, 0)[0]
            typ = payload[4]
            return {'service': 'ping', 'seq': seq, 'type': typ}
    return {'service': f'svc_{service_id:02x}', 'raw': data.hex()}


def main():
    dev = usb.core.find(idVendor=VENDOR, idProduct=PRODUCT)
    if dev is None:
        print("PRIME 4 Screen not found. Is the device in Computer Mode?")
        sys.exit(1)

    print(f"Found PRIME 4 Screen: bus={dev.bus} dev={dev.address}")

    # Step 1: SET_CONFIGURATION (once - triggers ffs_func_set_alt)
    try:
        if dev.is_kernel_driver_active(0):
            dev.detach_kernel_driver(0)
    except: pass
    dev.set_configuration()
    print("SET_CONFIGURATION sent")
    time.sleep(0.5)

    # Step 2: Claim interface
    usb.util.claim_interface(dev, 0)
    print("Interface 0 claimed")

    received = []
    stop = threading.Event()

    # Step 3: Keep IN endpoints polled continuously
    def reader(ep, sz, name):
        while not stop.is_set():
            try:
                d = dev.read(ep, sz, timeout=300)
                parsed = parse_response(bytes(d))
                received.append((name, bytes(d), parsed))
                print(f"\n  *** {name} ({len(d)}B): {bytes(d).hex()}")
                if parsed:
                    print(f"      parsed: {parsed}")
            except usb.core.USBTimeoutError:
                pass
            except Exception as e:
                if not stop.is_set():
                    print(f"  {name} error: {e}")
                break

    for ep, sz, name in [(EP_SMEX_IN, 64, "EP3-intr-IN"), (EP_BULK_IN, 512, "EP1-bulk-IN")]:
        t = threading.Thread(target=reader, args=(ep, sz, name), daemon=True)
        t.start()

    time.sleep(0.3)

    # Step 4: Send smex.protocolversion handshake
    print("\n--- Sending smex.protocolversion ---")
    msg = build_smex("smex.protocolversion", "1")
    print(f"  EP4 OUT: {msg.hex()}")
    dev.write(EP_SMEX_OUT, msg, timeout=2000)

    # Also send on bulk in case both are needed
    dev.write(EP_BULK_OUT, msg, timeout=2000)

    # Send a ping
    print("\n--- Sending ping ---")
    ping = build_ping(1)
    dev.write(EP_SMEX_OUT, ping, timeout=2000)

    print("\nWaiting 10s for responses...")
    time.sleep(10)
    stop.set()

    print(f"\nTotal received: {len(received)}")
    for name, raw, parsed in received:
        print(f"  {name}: {parsed or raw.hex()}")

    usb.util.release_interface(dev, 0)


if __name__ == "__main__":
    main()
