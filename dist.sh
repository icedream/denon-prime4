#!/bin/bash

set -e
set -u
set -o pipefail

rm -rf unpacked-img
./unpack.sh "$@"
./compile-buildroot.sh "$@"
./pack.sh "$@"
./unpack-updater.sh "$@"
./generate-updater-win.sh "$@"
