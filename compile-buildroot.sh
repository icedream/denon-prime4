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

# Version check for new gcc default language version to avoid build failures
if command -v gcc 2>/dev/null >/dev/null; then
  # read gcc version
  gcc_version=$(gcc --version | grep -Eo 'gcc.+[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+' | cut -d' ' -f 3)
  if [ "$( (echo "$gcc_version" && echo "14.0.0") | sort -V | head -n1 )" = "14.0.0" ]; then
    # gawk 5.3.0 and older
    #
    # ref #66
    # ref gawk 7a521fe4b37f8554ca53ef3236f0352e391aaa1d (contained in 5.3.1)
    if [ -f "${buildroot_path}/package/gawk/gawk.mk" ]; then
      gawk_version=$(cat "${buildroot_path}/package/gawk/gawk.mk" | grep -Po '^GAWK_VERSION\s*=\s*\K.+$')
      if [ "$( (echo "$gawk_version" && echo "5.3.1") | sort -V | head -n1 )" != "5.3.1" ]; then
        # do not default to c23+
        make_flags+=(HOST_GAWK_CONF_ENV=CFLAGS=-std=gnu17)
        echo "WARNING: Setting CFLAGS=-std=gnu17 as the found gcc $gcc_version is too new for gawk $gawk_version and would default to wrong language version"
      fi
    fi
  fi
fi

make "${make_flags[@]}"
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
