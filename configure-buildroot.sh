#!/bin/bash -e

#./clone-buildroot.sh
cp -v buildroot-config/.config buildroot/*/

config_target=nconfig
if [ -n "$DISPLAY" ]
then
  config_target=xconfig
fi

make -C buildroot/*/ -j$(nproc) BR2_EXTERNAL=../../buildroot-customizations "$config_target"
cp -v buildroot/*/.config buildroot-config
