#!/bin/bash -e

. ./functions.sh

if ! command -v dumpimage >/dev/null; then
  log_fatal "You need u-boot-tools installed (dumpimage command seems to be missing)."
fi

files=("${prime4_update_download_filename}")

# Replaces the full data string with a reference to the extracted image file.
patch_dts() {
  sed \
    -e '/^\s\+data = <.\+>;/d' \
    -e '/^\s\+value = <.\+>;/d' \
    -e 's,^\(\s\+\)partition = "\(.\+\)";$,\1partition = "\2";\n\1data = /incbin/("'"$output_dir"'/\2.img.xz");,g' \
    -u
}

download_firmware() {
  log "*** Downloading ${prime4_update_download_filename}"
  curl '-#Lo' "${prime4_update_download_filename}" "${prime4_update_download_url}"
  files+=("${prime4_update_download_filename}")
}

for file in "${files[@]}"; do
  if [ ! -f "$file" ]; then
    #log_fatal "Need $file. Put it into the current working directory ($(pwd))."
    download_firmware
  fi

  device_id=$(dumpimage -l "$file" | grep '^FIT description:' | cut -d: -f2 | awk '{print $1}')
  output_dir="unpacked-img/$device_id"

  #log "*** Extracting kernel and DTB"
  #extract-dtb "$file" -o "$output_dir"/

  #for dtb in "$output_dir"/*.dtb; do
  for dtb in "$file"; do
    if [ ! -f "$dtb.dts" ]; then
      log "*** Converting $dtb to DTS, this can take a few minutes"
      dtc -I dtb -O dts "$dtb" | patch_dts >"$dtb.dts"
      continue
    else
      log "*** Skipping conversion of $dtb to DTS, file $dtb.dts already exists"
    fi

    log "*** Unpacking $dtb"
    mkdir -p "$output_dir"
    dumpimage -l "$dtb"
    dumpimage -T flat_dt -p 0 -o "$output_dir"/splash.img.xz "$dtb"
    rm -f "$output_dir"/splash.img
    xz -vd "$output_dir"/splash.img.xz
    dumpimage -T flat_dt -p 1 -o "$output_dir"/recoverysplash.img.xz "$dtb"
    rm -f "$output_dir"/recoverysplash.img
    xz -vd "$output_dir"/recoverysplash.img.xz
    dumpimage -T flat_dt -p 2 -o "$output_dir"/rootfs.img.xz "$dtb"
    rm -f "$output_dir"/rootfs.img
    xz -vd "$output_dir"/rootfs.img.xz
  done
done
