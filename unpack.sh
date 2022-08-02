#!/bin/sh -e

prime4_update_download_url="https://imb-cicd-public.s3.amazonaws.com/Engine/2.2.2/Release/EOS/435be138f0/PRIME4-2.2.2-Update.img"
prime4_update_download_filename="${prime4_update_download_url##*/}"

log() {
  echo "$@" >&2
}

log_fatal() {
  echo "ERROR:" "$@" >&2
  exit 1
}

if ! command -v dumpimage >/dev/null; then
  log_fatal "You need u-boot-tools installed (dumpimage command seems to be missing)."
fi

files=( $(find -mindepth 1 -maxdepth 1 -name \*.img ) )

# Replaces the full data string with a reference to the extracted image file.
patch_dts() {
  sed \
    -e '/^\s\+data = <.\+>;/d' \
    -e '/^\s\+value = <.\+>;/d' \
    -e 's,^\(\s\+\)partition = "\(.\+\)";$,\1partition = "\2";\n\1data = /incbin/("unpacked-img/\2.img.xz");,g' \
    -u
}

download_firmware() {
  log "*** Downloading ${prime4_update_download_filename}"
  curl '-#Lo' "${prime4_update_download_filename}" "${prime4_update_download_url}"
  files+=( "${prime4_update_download_filename}" )
}

if [ "${#files[@]}" -lt 1 ]; then
  #log_fatal "Need at least one .img file to process. Put it into the current working directory ($(pwd))."
  download_firmware
fi

for file in "${files[@]}"; do
  #log "*** Extracting kernel and DTB"
  #extract-dtb "$file" -o unpacked-img/

  #for dtb in unpacked-img/*.dtb; do
  for dtb in "$file"; do
    if [ ! -f "$dtb.dts" ]
    then
      log "*** Converting $dtb to DTS, this can take a few minutes"
      dtc -I dtb -O dts "$dtb" | patch_dts > "$dtb.dts"
      continue
    else
      log "*** Skipping conversion of $dtb to DTS, file $dtb.dts already exists"
    fi

    log "*** Unpacking $dtb"
    mkdir -p unpacked-img
    dumpimage -l "$dtb"
    dumpimage -T flat_dt -p 0 -o unpacked-img/splash.img.xz "$dtb"
    rm -f unpacked-img/splash.img
    xz -vd unpacked-img/splash.img.xz
    dumpimage -T flat_dt -p 1 -o unpacked-img/recoverysplash.img.xz "$dtb"
    rm -f unpacked-img/recoverysplash.img
    xz -vd unpacked-img/recoverysplash.img.xz
    dumpimage -T flat_dt -p 2 -o unpacked-img/rootfs.img.xz "$dtb"
    rm -f unpacked-img/rootfs.img
    xz -vd unpacked-img/rootfs.img.xz
  done
done
