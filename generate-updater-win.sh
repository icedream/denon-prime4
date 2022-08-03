#!/bin/bash

. ./functions.sh

lzma_sdk_url="https://www.7-zip.org/a/lzma2107.7z"
lzma_sdk_filename="${lzma_sdk_url##*/}"

if ! command -v 7z >/dev/null; then
  log_fatal "You need 7-zip installed (7z command seems to be missing)."
fi

sfx=( $(find -mindepth 1 -maxdepth 1 -name \*.sfx ) )

download_sfx() {
  log "*** Downloading ${lzma_sdk_filename}"
  curl '-#Lo' "${lzma_sdk_filename}" "${lzma_sdk_url}"
  log "*** Unpacking SFX from ${lzma_sdk_filename}"
  7z e -y -o. "${lzma_sdk_filename}" bin/7zS2.sfx
  sfx+=(7zS2.sfx)
}

if [ "${#sfx[@]}" -lt 1 ]; then
  download_sfx
fi
if [ "${#sfx[@]}" -gt 1 ]; then
  log_warning "More than one .sfx file found, using the first one: ${sfx[1]}"
fi

files=( $(find -mindepth 1 -maxdepth 1 -name \*.dtb ) )

if [ "${#files[@]}" -lt 1 ]; then
  log_fatal "Need at least one .dtb file to process. Generate it with ./pack.sh or put it into the current working directory ($(pwd))."
fi

for file in "${files[@]}"; do
  cp -v "${file}" updater/win/update.img
  dtb_name="$(basename "${file}" .dtb)"
  for sfx_file in "${sfx[@]}"; do
    sfx_name="$(basename "$sfx_file" .sfx)"
    exe_name="${dtb_name}_${sfx_name}.exe"
    archive_name="${dtb_name}_${sfx_name}.7z"
    echo "*** Packing updater files"
    7z a "$archive_name" ./updater/win*
    trap 'rm -f "$archive_name"' EXIT
    echo "*** Generating ${exe_name} with ${sfx}"
    cat "${sfx_file}" sfx-config.txt "$archive_name" >"$exe_name"
  done
done
