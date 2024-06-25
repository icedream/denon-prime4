#!/bin/bash -e

. ./functions.sh

buildroot_path="buildroot/$(get_buildroot_version)"

#./clone-buildroot.sh

config_target="${1:-}"
if [ -z "$config_target" ]; then
  if [ -n "${DISPLAY:-}" ]
  then
    config_target=xconfig
  else
    config_target=nconfig
  fi
fi

make_flags=(
  -C "${buildroot_path}"
  BR2_EXTERNAL=../../buildroot-customizations
  BR2_DEFCONFIG="${SCRIPT_DIR}/buildroot-customizations/configs/${device_id_lowercase}_defconfig"
)

make \
  "${make_flags[@]}" \
  "${device_id_lowercase}_defconfig"
make \
  "${make_flags[@]}" \
  "$config_target"
make \
  "${make_flags[@]}" \
  savedefconfig
