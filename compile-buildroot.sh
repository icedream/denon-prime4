#!/bin/bash -e
packages=(
  chocolate-doom
  doom-wad
  openssh
)

filter_package_files() {
  filter_str=''
  for package in "${packages[@]}"; do
    if [ -n "$filter_str" ]; then
      filter_str="$filter_str"'\|'
    fi
    filter_str="$filter_str"'^'"$package,"
  done

  grep "$filter_str" | tr ',' ' ' | awk '{print $2}'
}

# remove spaces since buildroot does not like that
export PATH="${PATH// /}"

./clone-buildroot.sh
make -C buildroot/*/ -j$(nproc)
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
