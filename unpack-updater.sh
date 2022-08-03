#!/bin/sh -e

. ./functions.sh

if ! command -v 7z >/dev/null; then
  log_fatal "You need 7-zip installed (7z command seems to be missing)."
fi

files=("${prime4_updater_win_download_filename}")

download_updater_win() {
  log "*** Downloading ${prime4_updater_win_download_filename}"
  curl '-#Lo' "${prime4_updater_win_download_filename}" "${prime4_updater_win_download_url}"
  files+=("${prime4_updater_win_download_filename}")
}

for file in "${files[@]}"; do
  if [ ! -f "$file" ]; then
    #log_fatal "Need $file. Put it into the current working directory ($(pwd))."
    download_updater_win
  fi

  output_dir="updater/$device_id/win"

  log "*** Unpacking $file to $output_dir"
  mkdir -p "$output_dir"
  7z x -y -o"$output_dir" '-x!*.img' "$file"
done
