#!/usr/bin/bash -e

set -e
set -u
set -o pipefail

. ./functions.sh

name=$(basename "${device_update_download_filename}" .img)

version="$(git describe --tags --always | sed -e 's/^v//')"

prefix="${device_application_name}-${version}-Update"

ln -vfs "${device_update_download_filename}_7zS2.exe" "${prefix}.exe"
ln -vfs "${device_update_download_filename}.dtb" "${prefix}.img"
ln -vfs "${device_update_download_filename}.run" "${prefix}.run"
ln -vfs "${device_update_download_filename}_newupdater.exe" "${prefix}_new.exe"
