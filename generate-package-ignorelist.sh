#!/bin/bash
set -e

sudo ./mount.sh --list >file-list.txt
while read -r package filepath; do
  if grep -qF "$filepath" file-list.txt; then
    echo "$package"
  fi
done < <(cat buildroot/*/output/build/packages-file-list.txt | tr ',' ' ') | uniq
