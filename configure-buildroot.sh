#!/bin/bash -e

#./clone-buildroot.sh
cp -v buildroot-config/.config buildroot/*/

config_target="${1:-}"
if [ -z "$config_target" ]; then
  if [ -n "$DISPLAY" ]
  then
    config_target=xconfig
  else
    config_target=nconfig
  fi
fi
make -C buildroot/*/ -j$(nproc) BR2_EXTERNAL=../../buildroot-customizations "$config_target"
cp -v buildroot/*/.config buildroot-config
