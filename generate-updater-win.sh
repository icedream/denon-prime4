#!/bin/bash

. ./functions.sh

lzma_sdk_url="https://www.7-zip.org/a/lzma2107.7z"
lzma_sdk_filename="${lzma_sdk_url##*/}"

if ! command -v 7z >/dev/null; then
  log_fatal "You need 7-zip installed (7z command seems to be missing)."
fi

sfx=($(find -mindepth 1 -maxdepth 1 -name \*.sfx))

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

files=($(find -mindepth 1 -maxdepth 1 -name \*.dtb))

if [ "${#files[@]}" -lt 1 ]; then
  log_fatal "Need at least one .dtb file to process. Generate it with ./pack.sh or put it into the current working directory ($(pwd))."
fi

make -C go all-windows-amd64

tempdir=$(mktemp -d)
trap 'rm -rf ${tempdir}' EXIT

for file in "${files[@]}"; do
  dtb_name="$(basename "${file}" .dtb)"
  dtb_dir="$(dirname "$dtb_name")"

  # generate config file
  cat >"$tempdir"/config.toml <<EOF
[[devices]]
name = "${device_name}"
imagePath = "${dtb_name}.dtb"
usbConfig = 1
usbInterface = 0
usbAlternate = 0
usbInputEndpoint = 1
usbOutputEndpoint = 2
usbReadSize = 256
usbReadBufferSize = 0
usbWriteSize = 4096
usbWriteBufferSize = 0
usbOpTimeout = "1m"
EOF
  for sfx_file in "${sfx[@]}"; do
    sfx_name="$(basename "$sfx_file" .sfx)"
    exe_name="${dtb_name}_${sfx_name}.exe"
    archive_name="${dtb_name}_${sfx_name}.7z"
    echo "*** Packing updater files"
    # NOTE - keep ./ to make 7z strip dir paths
    7z a "$archive_name" ./go/updater.exe ./go/*.dll "$tempdir"/./config.toml "${dtb_dir}"/./"${dtb_name}".dtb
    trap 'rm -f "$archive_name"' EXIT
    echo "*** Generating ${exe_name} with ${sfx_file}"
    cat "${sfx_file}" sfx-config.txt "$archive_name" >"$exe_name"
  done
done
