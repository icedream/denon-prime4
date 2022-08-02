#!/bin/sh -e

prime4_updater_win_download_url="https://imb-cicd-public.s3.amazonaws.com/Engine/2.2.2/Release/EOS/435be138f0/Prime+4+2.2.2+Updater.exe"
prime4_updater_win_download_filename="${prime4_updater_win_download_url##*/}"

log() {
  echo "$@" >&2
}

log_fatal() {
  echo "ERROR:" "$@" >&2
  exit 1
}

if ! command -v 7z >/dev/null; then
  log_fatal "You need 7-zip installed (7z command seems to be missing)."
fi

files=( $(find -mindepth 1 -maxdepth 1 -name \*Updater.exe ) )

download_updater_win() {
  log "*** Downloading ${prime4_updater_win_download_filename}"
  curl '-#Lo' "${prime4_updater_win_download_filename}" "${prime4_updater_win_download_url}"
  files+=( "${prime4_updater_win_download_filename}" )
}

if [ "${#files[@]}" -lt 1 ]; then
  #log_fatal "Need at least one Updater.exe file to process. Put it into the current working directory ($(pwd))."
  download_updater_win
fi

for file in "${files[@]}"; do
  log "*** Unpacking $file to updater/win"
  mkdir -p updater/win
  7z x -y -oupdater/win '-x!*.img' "$file"
done
