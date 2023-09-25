#!/bin/bash -e

. ./functions.sh

buildroot_path="buildroot/$(get_buildroot_version)"

#./clone-buildroot.sh

config_target="${1:-}"
if [ -z "$config_target" ]; then
  if [ -n "$DISPLAY" ]; then
    config_target=xconfig
  else
    config_target=nconfig
  fi
fi

make -C "$buildroot_path" -j$(nproc) BR2_EXTERNAL=../../buildroot-customizations "$config_target"
