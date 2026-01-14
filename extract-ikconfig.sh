#!/bin/bash -e

. ./functions.sh

buildroot_path="buildroot/$(get_buildroot_version)"

# TODO - detect linux-headers version

make -C "${buildroot_path}" linux-headers

do_mount cat /boot/zImage >zImage
"${buildroot_path}"/output/build/linux-headers-*/scripts/extract-ikconfig zImage >"buildroot-customizations/board/inmusic/common/linux.config"
