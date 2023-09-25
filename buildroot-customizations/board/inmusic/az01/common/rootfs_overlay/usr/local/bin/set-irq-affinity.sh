#!/bin/sh

IRQ=$1
CPU=$2
echo "Setting" $IRQ "to CPU: "$CPU
grep $IRQ /proc/interrupts | cut -d':' -f1 | tr -d " \t" | xargs -t -i sh -c 'echo $1 > /proc/irq/$2/smp_affinity_list' -- $CPU {}
