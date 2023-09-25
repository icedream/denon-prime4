#!/bin/bash -e

. ./functions.sh

buildroot_path="buildroot/$(get_buildroot_version)"

# TODO - detect linux-headers version

do_mount cat /boot/zImage > zImage
"${buildroot_path}"/output/build/linux-headers-*/scripts/extract-ikconfig zImage > buildroot-customizations/board/inmusic/az01/common/kernel.config
