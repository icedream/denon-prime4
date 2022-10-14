#!/bin/bash

set -e
set -u
set -o pipefail

rm -rf unpacked-img
./unpack.sh "$@"
./compile-buildroot.sh "$@"
./pack.sh "$@"
./generate-updater-win.sh "$@"
