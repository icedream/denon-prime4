#!/bin/bash -e

. ./functions.sh

buildroot_path="buildroot/$(get_buildroot_version)"

#./clone-buildroot.sh
cp -v buildroot-config/.config "$buildroot_path"

config_target="${1:-}"
if [ -z "$config_target" ]; then
  if [ -n "$DISPLAY" ]
  then
    config_target=xconfig
  else
    config_target=nconfig
  fi
fi

BR2_GLOBAL_PATCH_DIR=""
for d in common "${device_id_lowercase}"; do
  if [ -d "${SCRIPT_DIR}/buildroot-customizations/board/inmusic/$d/patches" ]; then
    if [ -n "${BR2_GLOBAL_PATCH_DIR}" ]; then
      BR2_GLOBAL_PATCH_DIR="${BR2_GLOBAL_PATCH_DIR} "
    fi
    BR2_GLOBAL_PATCH_DIR="${BR2_GLOBAL_PATCH_DIR}${SCRIPT_DIR}/buildroot-customizations/board/inmusic/$d/patches"
  fi
done

make_flags=(
  -C "${buildroot_path}"
  BR2_EXTERNAL=../../buildroot-customizations
  BR2_GLOBAL_PATCH_DIR="${BR2_GLOBAL_PATCH_DIR}"
)

make \
  "${make_flags[@]}" \
  "$config_target"
cp -v "$buildroot_path/.config" buildroot-config
