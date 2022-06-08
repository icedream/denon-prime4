#!/bin/bash -e

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
  for package in "${ignored_packages[@]}"; do
    if [ "$package" = "$1" ]; then
      return 0
    fi
  done
  return 1
}

filter_package_files() {
  while read -r package filepath; do
    if is_ignored_package "$package"; then
      # file from a ignored package, skip
      continue
    fi
    if [ ! -f buildroot/*/output/target/"$filepath" ]; then
      # file is not included in actual generated rootfs (e.g. header/docs/...), skip
      continue
    fi
    echo "$filepath"
  done < <(grep "$filter_str" | tr ',' ' ')
}

# remove spaces since buildroot does not like that
export PATH="${PATH// /}"

#./clone-buildroot.sh
#make -C buildroot/*/ -j$(nproc)
tar -c -v -C buildroot/*/output/target/ --owner=root --group=root \
	$(cat buildroot/*/output/build/packages-file-list.txt | filter_package_files) \
	| \
sudo ./mount.sh --write tar -xp
sudo ./mount.sh --write systemctl enable sshd
if ! sudo ./mount.sh grep -q sshd /etc/group; then
  sudo ./mount.sh --write /sbin/addgroup -S sshd
fi
if ! sudo ./mount.sh grep -q sshd /etc/passwd; then
  sudo ./mount.sh --write /sbin/adduser -H -S -D -G sshd -h /var/empty sshd
fi
sudo ./mount.sh --write sed -i 's,#PermitRootLogin .\+,PermitRootLogin yes,g' /etc/ssh/sshd_config
(echo denonprime4 && echo denonprime4) | sudo ./mount.sh --write passwd root
sudo ./mount.sh --write mkdir -p /var/empty
