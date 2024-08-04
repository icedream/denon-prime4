#!/bin/bash

set -e
set -u
set -o pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

log() {
  echo "$@" >&2
}

log_debug() {
  [ "${DEBUG:-0}" -eq 0 ] || echo "$@" >&2
}

log_fatal() {
  echo "ERROR:" "$@" >&2
  exit 1
}

log_warning() {
  echo "WARNING:" "$@" >&2
}

get_buildroot_version() {
  trap 'rm -f usr/lib/os-release; rmdir usr/lib usr || true' EXIT
  7z x -o. "$unpacked_img_dir"/rootfs.img usr/lib/os-release >/dev/null

  . ./usr/lib/os-release
  printf '%s' "${VERSION_ID}"
}

do_mount() {
  sudo ./mount.sh -d "$device" -v "$vendor" "$@"
}

vendor="${ENGINEOS_VENDOR:-denon}"
device="${ENGINEOS_DEVICE:-prime4}"
proc_args=()

while [ "$#" -gt 0 ]; do
  case "$1" in
  -d | --device)
    device="$2"
    shift 2
    ;;
  -v | --vendor)
    vendor="$2"
    shift 2
    ;;
  --)
    proc_args+=("$@")
    break
    ;;
  *)
    proc_args+=("$1")
    shift 1
    ;;
  esac
done
set -- "${proc_args[@]}"
log_debug "Set arguments to:" "${proc_args[@]}"

export ENGINEOS_DEVICE="$device" ENGINEOS_VENDOR="$vendor"

device_id=
device_update_download_url=
device_update_download_filename=
device_application_name=
device_updater_win_download_url=
device_updater_win_download_filename=
device_name=

while read -r current_vendor current_device current_device_id current_device_application_name current_img_download_url current_updater_download_url current_device_name; do
  if [ "$current_vendor" = "$vendor" ] && [ "$current_device" = "$device" ]; then
    device_id="$current_device_id"
    device_update_download_url="$current_img_download_url"
    device_updater_win_download_url="${current_img_download_url%.img}.exe"
    if [ -n "$current_updater_download_url" ]; then
      device_updater_win_download_url="${current_updater_download_url}"
    fi
    device_application_name="$current_device_application_name"
    device_name="$current_device_name"
    break
  fi
done <devices.txt

if [ -z "$device_id" ]; then
  echo "ERROR: invalid vendor or device." >&2
  exit 1
fi

device_id_lowercase=$(tr '[[:upper:]]' '[[:lower:]]' <<<"$device_id")

device_update_download_filename="${device_update_download_url##*/}"
device_updater_win_download_filename="${device_updater_win_download_url##*/}"
device_updater_win_download_filename="${device_updater_win_download_filename//+/ }"

unpacked_img_dir="unpacked-img/$device_id"
