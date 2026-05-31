# FunctionFS Kernel Issue - Prime 4 Computer Mode USB Communication

## Problem

The Prime 4 running firmware 4.3.4 (kernel `6.1.111-inmusic-2024-09-19-rt41`) has a critical bug in the dwc2 USB OTG controller driver or FunctionFS layer that prevents proper USB endpoint I/O completion signaling.

## Symptoms

When `planck-remote-screen` is running in Computer Mode on the Prime 4:

1. Host writes to EP4 OUT (interrupt, 64B) or EP2 OUT (bulk, 512B) succeed from libusb perspective
2. Data lands in the dwc2 DMA buffer (confirmed by reading `/dev/mem` at the DOEPDMA address)
3. USB interrupts fire (IRQ counter increments by 1 per write)
4. But the FunctionFS layer never signals completion to userspace:
   - `total_data=0` in `cat /sys/kernel/debug/usb/ff580000.usb/ep[24]out`
   - Any process attempting to read or write FunctionFS endpoint fds blocks forever
   - AIO submit requests (PREAD via io_submit) hang in `ffs_epfile_io` at `wait_event_interruptible`

## Root Cause

The dwc2 USB controller receives OUT transfers correctly into DMA buffers and triggers hardware interrupts, but the dwc2 driver is not properly invoking the completion callback for FunctionFS. The FunctionFS layer thus has no way to know that data arrived, so:

- Userspace applications waiting via poll/epoll on endpoint fds never get woken up
- Userspace applications using AIO hang waiting for the completion event
- Direct read/write calls on the endpoint fds block indefinitely

This manifests as a complete communications deadlock: the device receives data but the application cannot retrieve it.

## Hardware/Software Stack

- **SoC**: Rockchip RK3288 (ARM Cortex-A17)
- **USB OTG Controller**: DWC2 (Synopsys DesignWare Cores 2)
- **Gadget Framework**: Linux FunctionFS
- **Kernel**: `6.1.111-inmusic-2024-09-19-rt41` (custom build, PREEMPT_RT enabled)
- **Device-side app**: `/usr/bin/planck-remote-screen` (11 MB JUCE app)

## Verification

### Data arrives in DMA buffer but not retrieved

```bash
# On Prime 4, write test message from host
# Then check the DMA buffer directly:
DOEPDMA=0x038afdc0  # from cat /sys/kernel/debug/usb/ff580000.usb/ep4out
hexdump -C /dev/mem -s $DOEPDMA -n 64
# Output: confirms message bytes are present in physical memory
```

### IRQ increments without completion callback

```bash
# Watch IRQ count before and after host write
grep "ff580000" /proc/interrupts     # Before: count=191
# [host writes to EP4 OUT]
grep "ff580000" /proc/interrupts     # After: count=192 (+1)

# But check the request status:
cat /sys/kernel/debug/usb/ff580000.usb/ep4out
# total_data=0  <-- kernel thinks no completion happened
```

### Read/Write operations hang

```bash
# Any of these block forever:
cat /dev/ffs-smexstream/ep4       # blocks indefinitely
read(fd_ep4, buf, 64)             # blocks indefinitely  
write(fd_ep3, buf, 64)            # blocks indefinitely
io_submit(ctx, 1, &read_iocb)    # blocks indefinitely
```

## Workarounds Considered

### 1. Use Network Mode Instead of USB FunctionFS

**Status**: Unknown feasibility

`planck-remote-screen` binary contains two transport implementations:
- `UsbGadgetMessageBlockStream` (FunctionFS - broken)
- `NetworkMessageBlockStream` (TCP - may work)

If the app can be configured or patched to use TCP port X instead of FunctionFS, the SMEX protocol could work over Ethernet.

**Action needed**: Reverse engineer whether planck-remote-screen can be configured to listen on a TCP port, or patch the binary to enable this.

### 2. Kernel Patch / Update

**Status**: Requires local kernel dev environment

The dwc2 and FunctionFS drivers need review to identify why:
- dwc2 is not calling completion callbacks for OUT transfers, OR
- FunctionFS is not propagating the callbacks to poll/epoll/AIO

**Action needed**: 
- Check dwc2 git history for fixes between 6.1.111 and latest
- Review RK3288 dwc2 configuration for known issues
- Test on stock 6.1.111 or upgraded kernel (6.8+)

### 3. Use a Different USB Profile

**Status**: Infeasible for screen display

Currently Prime 4 in Computer Mode uses a vendor-specific FunctionFS gadget (`0x15e4:0xa008`, smexstream). Could instead use:
- CDC ACM (serial port emulation) - but slower and less suitable for display data
- Mass storage - not applicable
- Custom USB driver - requires kernel recompilation

### 4. Local IPC Instead of Remote USB

**Status**: Architecture change required

For Mixxx integration running on the Prime 4 itself (not over USB), bypass planck-remote-screen entirely and write directly to the framebuffer or render surface. But this breaks the intended remote-display architecture.

## Current Status

**Communication is blocked**: No DJ software (Serato, Engine DJ, Mixxx, etc.) can successfully control the Prime 4 screen from a remote computer in Computer Mode due to this kernel bug.

The "waiting for connection" UI displayed by planck-remote-screen is a built-in fallback animation, not the result of receiving any command.

## Next Steps (Priority Order)

1. **Network Mode Testing**: Inspect planck-remote-screen binary for TCP network support and test if it works
2. **Kernel Upgrade**: Test on a newer kernel (6.8 or later) to see if dwc2/FunctionFS fixes apply
3. **Targeted Patch**: If a fix is identified, create a minimal kernel patch for 6.1.111
4. **Binary Patching**: If kernel changes are infeasible, attempt to reverse engineer and patch planck-remote-screen to disable FunctionFS and use TCP instead
5. **Contact Denon**: Report this as a critical bug in the Computer Mode feature

## References

- dwc2 driver source: `drivers/usb/dwc2/` in Linux kernel
- FunctionFS implementation: `drivers/usb/gadget/function/f_fs.c`
- Prime 4 rootfs: `/usr/bin/planck-remote-screen` (JUCE app with RTTI strings)
- RK3288 OTG controller: `ff580000.usb` (address shown in dmesg)

## Test Environment

- Prime 4 firmware: JP11 (v4.3.4 custom)
- Host machine: linux laptop (icedream, 192.168.187.22)
- Server machine: icedream-homepc (192.168.187.21, Prime 4 physical connection)
- Prime 4 IP: 192.168.187.23
