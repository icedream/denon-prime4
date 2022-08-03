#!/bin/bash

. ./functions.sh

if ! command -v dtc >/dev/null; then
  log_fatal "dtc command seems to be missing. You need to install the device-tree-compiler for this script to work."
fi

files=("${prime4_update_download_filename}.dts")

files_to_delete=()
on_exit() {
  for file in "${files_to_delete[@]}"; do
    rm -f "${file}"
  done
}
trap 'on_exit' EXIT
for img in "$unpacked_img_dir"/*.img; do
  if [ -f "$img.xz" ]; then
    log "$img.xz already exists, skipping."
    continue
  fi
  log "*** Compressing $img to $img.xz"
  #xz -vk9eT0 --check=crc64 "$img"
  #sha1sum "$img.xz" | awk '{print $1}' | xxd -r -p >"$img.xz.sha1"
  make "$img.xz" "$img.xz.sha1"
  files_to_delete+=("$img.xz" "$img.xz.sha1")
done

for file in "${files[@]}"; do
  if [ ! -f "$file" ]; then
    log_fatal "Need $file to process in either the current working directory or $unpacked_img_dir."
  fi

  dtb="$(basename "$file" .dts).dtb"

  log "*** Generating FIT $dtb"
  mkimage -f "$file" "$dtb"
done
