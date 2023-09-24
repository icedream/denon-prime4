#!/bin/sh

APPNAME=$1

# Increase priority to max for audio IRQ
grep -h irq/.*ffb20000 /proc/*/stat 2>/dev/null | cut -d' ' -f1 | xargs -i chrt -f -p 99 {}
# Setting GPU to CPU 1
set-irq-affinity.sh ffa30000 1
# Setting Audio to CPU 3
set-irq-affinity.sh ffb20000 3
# Setting Network to CPU 1
set-irq-affinity.sh eth0 1

if [ ! -e /sys/firmware/devicetree/base/serial@ff190000/control-surface ];
then
	# Control surface is on USB (not UART) so setting USB to CPU 1:
	set-irq-affinity.sh ehci_hcd 1
	set-irq-affinity.sh ff540000 1
fi

# Enable external SD Card interface
echo external > /sys/bus/platform/devices/sd-mux/state

# Set GPU governor to performance mode
if [ $APPNAME == "JP21" ];
then
	echo simple_ondemand > /sys/devices/platform/ffa30000.gpu/devfreq/ffa30000.gpu/governor
else
	echo performance > /sys/devices/platform/ffa30000.gpu/devfreq/ffa30000.gpu/governor
fi

# Set midi output buffer size to max
echo 524287 > /sys/module/snd_seq_midi/parameters/output_buffer_size

# Disable vsync_on_pan, with the new Linux kernel vsync is on by default,
# and this line would add a double vsync, thereby halving the framerate.
# vsync_on_pan is a mechanism introduced by John Keeping
echo 0 >/sys/class/graphics/fb0/vsync_on_pan

# supressing debug output to avoid audio dropouts on USB/SD device plugging
dmesg -n crit
