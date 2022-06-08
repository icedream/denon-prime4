#!/bin/sh -e

log() {
  echo "$@" >&2
}

log_fatal() {
  echo "ERROR:" "$@" >&2
  exit 1
}

trap 'rm -f usr/lib/os-release; rmdir usr/lib usr || true' EXIT
7z x -o. unpacked-img/rootfs.img usr/lib/os-release

. ./usr/lib/os-release

# Now following variables will be set:
#
# NAME=Buildroot
# VERSION=2021.02.9-83-g1f864943a0
# ID=buildroot
# VERSION_ID=2021.02.10
# PRETTY_NAME="Buildroot 2021.02.10"

git init "buildroot/${VERSION_ID}"
(
  cd "buildroot/${VERSION_ID}"
  git remote add origin https://git.buildroot.net/buildroot || true
  git fetch origin "refs/tags/${VERSION_ID}:refs/tags/${VERSION_ID}"
  git checkout "${VERSION_ID}"
  patches_dir=../../buildroot-patches/"${VERSION_ID}"
  if [ -d "${patches_dir}" ]; then
    git am "${patches_dir}"/*.patch
  fi
)

cp -rv buildroot-config/. buildroot/"${VERSION_ID}"
