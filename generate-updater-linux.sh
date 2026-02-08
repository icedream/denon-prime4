#!/bin/bash

. ./functions.sh

if ! command -v makeself >/dev/null; then
  log_fatal "You need makeself installed (makeself command seems to be missing)."
fi

files=("${device_update_download_filename}.dtb")

if [ "${#files[@]}" -lt 1 ]; then
  log_fatal "Need at least one .dtb file to process. Generate it with ./pack.sh or put it into the current working directory ($(pwd))."
fi

make -C go updater copy-libs-updater

tempdir=$(mktemp -d)
trap 'rm -rf ${tempdir}' EXIT

for file in "${files[@]}"; do
  dtb_name="$(basename "${file}" .dtb)"
  dtb_dir="$(dirname "$(readlink -f "$file")")"

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
  run_name="${dtb_name}.run"
  archive_dir="${dtb_name}_linux"
  startup_script="./updater.sh"
  mkdir -v "$archive_dir"
  trap 'rm -rf "$archive_dir"' EXIT
  (
    cd "$archive_dir"
    ln -vs ../go/updater
    for so in ../go/*.so*; do
       ln -vs "$so"
    done
    ln -vs "$tempdir"/config.toml
    ln -vs "${dtb_dir}/${dtb_name}.dtb"
    cat >"$startup_script" <<'EOF'
#!/usr/bin/env sh
export LD_LIBRARY_PATH="$(pwd):${LD_LIBRARY_PATH}"
exec ./updater "$@"
EOF
    chmod -v +x "$startup_script"
  )
  "${MAKESELF:-makeself}" --nocomp --follow "$archive_dir" "$run_name" "${device_name} Firmware Updater" "$startup_script"
done
