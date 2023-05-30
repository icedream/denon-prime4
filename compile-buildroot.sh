#!/bin/bash -e

. ./functions.sh

# read in packages for which we do not want to modify files already shipped with original firmware
ignored_packages=()
while read -r package; do
  # remove comments
  package="${package%\#*}"
  # skip empty lines
  if [ -z "$package" ]; then
    continue
  fi
  ignored_packages+=("$package")
done <package-ignorelist.txt

is_ignored_package() {
  local package
  for package in "${ignored_packages[@]}"; do
    if [ "$package" = "$1" ]; then
      return 0
    fi
  done
  return 1
}

filter_package_files() {
  local package
  local filepath
  while read -r package filepath; do
    case "$filepath" in
    *.h|*.la|./usr/include/*|./usr/share/doc/*|./usr/share/man/*|./usr/lib/pkgconfig/*|./usr/lib/cmake/*)
      # docs/man files/headers, skip without logging
      continue
      ;;
    esac
    if is_ignored_package "$package"; then
      # file from a ignored package, skip
      echo "Ignoring file from $package (ignored package): $filepath" >&2
      continue
    fi
    if [ ! -f "${buildroot_path}/output/target/${filepath}" ]; then
      # file is not included in actual generated rootfs (e.g. header/docs/...), skip
      echo "Ignoring file from $package (deleted by buildroot): $filepath" >&2
      continue
    fi
    echo "$filepath"
    echo "Adding file from $package: $filepath" >&2
  done < <(tr ',' ' ')
}

# remove spaces since buildroot does not like that
export PATH="${PATH// /}"

./clone-buildroot.sh

buildroot_path="buildroot/$(get_buildroot_version)"

make_flags=(
  -C "${buildroot_path}"
  BR2_EXTERNAL=../../buildroot-customizations
)

if [ -n "${BR2_JLEVEL:-}" ]; then
  make_flags+=(BR2_JLEVEL="${BR2_JLEVEL}")
fi

if [ -n "${BR2_CCACHE_DIR:-}" ]; then
  make_flags+=(BR2_CCACHE_DIR="${BR2_CCACHE_DIR}")
fi

if [ -n "${BR2_CCACHE:-}" ]; then
  make_flags+=(BR2_CCACHE="${BR2_CCACHE}")
fi

failed=0
for i in $(seq 1 20); do
  if ! make "${make_flags[@]}"; then
    failed=$?
    continue
  fi
  failed=0
  break
done
if [ $failed -ne 0 ]; then
  echo "ERROR: make failed after multiple attempts. Aborting." >&2
  exit $failed
fi
# delete Engine, SoundSwitch and Qt files from original firmware
sudo ./mount.sh --write rm -rf \
  /etc/systemd/system/engine.service \
  /etc/systemd/system/basic.target.wants/az01-libmali-setup.service \
  /root/.screenrc \
  /usr/Engine \
  /usr/mkspecs \
  /usr/qml \
  /usr/qt \
  /usr/share/Akai \
  /usr/SoundSwitch
filter_package_files <"${buildroot_path}/output/build/packages-file-list.txt" | \
tar -c -C "${buildroot_path}/output/target/" --owner=root --group=root -T - |\
do_mount --write tar -xp
do_mount --write systemctl enable sshd
if ! do_mount grep -q sshd /etc/group; then
  do_mount --write /sbin/addgroup -S sshd
fi
if ! do_mount grep -q sshd /etc/passwd; then
  do_mount --write /sbin/adduser -H -S -D -G sshd -h /var/empty sshd
fi
do_mount --write sed -i 's,#PermitRootLogin .\+,PermitRootLogin yes,g' /etc/ssh/sshd_config
(echo denonprime4 && echo denonprime4) | do_mount --write passwd root
do_mount --write mkdir -p /var/empty
