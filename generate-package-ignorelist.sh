#!/bin/bash

. ./functions.sh

buildroot_path="buildroot/$(get_buildroot_version)"

sudo ./mount.sh --list >file-list.txt
while read -r package filepath; do
  if grep -qF "$filepath" file-list.txt; then
    echo "$package"
  fi
done < <(cat "$buildroot_path"/output/build/packages-file-list.txt | tr ',' ' ') | uniq
