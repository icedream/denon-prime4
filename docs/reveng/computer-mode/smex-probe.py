#!/usr/bin/env python3
"""
PRIME 4 Computer Mode - SMEX protocol probe script.

Protocol appears unidirectional, but we'll listen for responses:
  - EP4 OUT (0x04, interrupt, 64B) - control messages, MUST be exactly 64B
  - EP2 OUT (0x02, bulk, up to 4096B) - bulk data (images etc.)
  - EP3 IN  (0x83) and EP1 IN (0x81) - read attempts (may receive responses or empty)

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
  [uint32 BE: sequence ID]
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


def build_ping():
    """Build a SMEX PingPongService PING message."""
    # Sequence ID: incrementing counter (use current time for demo)
    seq_id = int(time.time() * 1000) & 0xFFFFFFFF
    inner = struct.pack('>I', seq_id)
    inner += bytes([0x01])  # type = ping
    return inner


def build_pong(seq_id):
    """Build a SMEX PingPongService PONG message."""
    inner = struct.pack('>I', seq_id)
    inner += bytes([0x03])  # type = pong
    return inner


def build_smex(type_str, payload=""):
    """Build a SMEX MessageBlock for EP4 OUT."""
    tb = type_str.encode('utf-8')
    pb = payload.encode('utf-8') if isinstance(payload, str) else payload
    inner = bytes([0x00])
    inner += struct.pack('>I', len(tb)) + tb
    inner += struct.pack('>I', len(pb)) + pb
    return struct.pack('>I', len(inner)) + inner


def read_from_in_endpoint(dev, ep_addr):
    """Read from an IN endpoint. Returns data bytes or None if timeout."""
    try:
        data = dev.read(ep_addr, 4096, timeout=1000)
        return data
    except usb.core.USBTimeoutError:
        return None
    except usb.core.USBError as e:
        if e.errno == 110:  # timeout
            return None
        raise


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

    # Set up reading from IN endpoints in a background thread
    import threading
    responses = []
    stop_reading = threading.Event()

    def read_in_endpoint_thread(ep_addr, name):
        """Background thread that continuously reads from an IN endpoint."""
        while not stop_reading.is_set():
            data = read_from_in_endpoint(dev, ep_addr)
            if data is not None and len(data) > 0:
                responses.append({
                    'ep': name,
                    'data': data,
                    'hex': data.hex()
                })
                print(f"\n[Received {name}]: {len(data)} bytes = {data.hex()}")
            time.sleep(0.01)  # Small sleep to avoid hogging CPU

    # Start background readers for EP1 IN and EP3 IN
    reader_threads = []
    for ep_addr, ep_name in [(0x81, 'EP1 IN'), (0x83, 'EP3 IN')]:
        t = threading.Thread(target=read_in_endpoint_thread, args=(ep_addr, ep_name), daemon=True)
        t.start()
        reader_threads.append(t)
    print("Started background readers for EP1 IN and EP3 IN")

    ## Send SMEX PingPong PING to initiate keepalive
    #print("\n--- Sending PingPong PING ---")
    #msg = pad64(build_ping())
    #print(f"  EP4 OUT (not padded): {msg.hex()}")
    ## PingPongService message, send without 64B padding
    #dev.write(EP_CTRL_OUT, msg, timeout=2000)
    #time.sleep(0.5)

    # Send smex.protocolversion (must be padded to 64B)
    print("\n--- Sending smex.protocolversion ---")
    msg = build_smex("smex.protocolversion", "1")
    print(f"  EP4 OUT (padded to 64B): {pad64(msg).hex()}")
    dev.write(EP_CTRL_OUT, pad64(msg), timeout=2000)
    time.sleep(0.2)

    ## Send smex.time.request
    #print("\n--- Sending smex.time.request ---")
    #msg = build_smex("smex.time.request")
    #print(f"  EP4 OUT (padded to 64B): {pad64(msg).hex()}")
    #dev.write(EP_CTRL_OUT, pad64(msg), timeout=2000)
    #time.sleep(0.2)

    while True:
        for b in ["50","100","150","200","250"]:
            # Send smex.brightness.set with value 50
            print(f"\n--- Sending smex.brightness.set = {b} ---")
            msg = build_smex("smex.brightness.set", b)
            print(f"  EP4 OUT (padded to 64B): {pad64(msg).hex()}")
            dev.write(EP_CTRL_OUT, pad64(msg), timeout=2000)
            time.sleep(0.5)
            dev.write(EP_CTRL_OUT, pad64(bytes([0])), timeout=2000)
            time.sleep(0.5)

    time.sleep(0.5)  # Wait for any responses

    # Stop background readers
    stop_reading.set()
    for t in reader_threads:
        t.join(timeout=1.0)

    # Display summary of responses
    print("\n" + "="*50)
    print("SUMMARY")
    print("="*50)
    if responses:
        print(f"\nReceived {len(responses)} response(s):")
        for resp in responses:
            print(f"  {resp['ep']}: {len(resp['data'])} bytes = {resp['hex']}")
    else:
        print("\nNo responses received")

    usb.util.release_interface(dev, 0)


if __name__ == "__main__":
    main()
