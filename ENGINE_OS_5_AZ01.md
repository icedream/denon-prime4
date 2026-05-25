# Engine OS 5.x AZ01 Notes

Engine OS 5.x no longer uses the older U-Boot FIT / DTB update image format
that this repository's original workflow expects. On tested Engine OS 5.0.1
PRIME GO firmware, the update image uses an `AZ01` container and the root
filesystem is Yocto/OpenEmbedded based rather than Buildroot based.

These notes are intentionally limited to observed behaviour from an original
Denon DJ PRIME GO (`JP11`) running Engine OS 5.0.1. They are not yet a complete
replacement for the existing unpack / modify / repack workflow.

## Observed System Details

- Device tested: original PRIME GO (`JP11`)
- Firmware tested: PRIMEGO 5.0.1
- Update image format: AZ01 container, not U-Boot FIT / DTB
- Rootfs reports: `az0x 5.0.14`
- Distro base: OpenEmbedded / Yocto, scarthgap
- Kernel observed on hardware: `6.6.119-az01-2025-12-17-rt67`
- Init system: systemd
- OpenSSH is already present in the rootfs

## SSH Enablement Findings

Enabling `sshd.service` through `/etc/systemd/system/multi-user.target.wants`
was not sufficient on the tested device. The likely reason is that `/etc` is
overlaid from the persistent system overlay under:

```text
/data/system/etc/overlay
```

Enabling the service from the vendor/rootfs unit directory did work:

```text
/usr/lib/systemd/system/multi-user.target.wants/sshd.service -> ../sshd.service
```

The following SSH configuration changes were also needed:

```text
PermitRootLogin yes
PasswordAuthentication yes
```

With those changes, the tested PRIME GO reported:

```text
systemctl status sshd
```

as active/running, and SSH login as `root` worked on hardware.

## Practical Notes

Do not expect the older Buildroot/FIT workflow to work unchanged on Engine OS
5.x images. The existing scripts that use `dumpimage`, generated DTS files, and
Buildroot 2021.02.10 are still relevant for older 2.x, 3.x, and 4.x images, but
Engine OS 5.x needs a separate AZ01 unpack/repack workflow.

The service enablement point above is useful once the rootfs can be modified and
repacked, but it does not by itself describe how to create a valid custom 5.x
update image.

## Tested PRIME GO SSH Image Procedure

This is the procedure that produced a working Engine OS 5.0.1 SSH-enabled
update image on an original PRIME GO (`JP11`). It is a small rootfs-only
modification, not the older Buildroot package build. The old FIT/DTB steps from
this repository were not usable because the image starts with `AZ01`, not a FIT
header.

Install the same basic tools used by the existing workflow:

```sh
sudo apt install xz-utils e2fsprogs
```

Use the 5.0.1 PRIME GO image. In my test tree I temporarily changed the
`primego` line in `devices.txt` to the 5.0.1 URL and let the existing download
logic fetch it, but the equivalent direct command is:

```sh
curl -L -o PRIMEGO-5.0.1-Update.img \
  'https://public.inmusiccdn.com/Engine/5.0.1/RELEASE/edad36dcec0e1f41/PRIMEGO-5.0.1-Update.img'
```

Confirm that the old FIT procedure is not applicable:

```sh
file PRIMEGO-5.0.1-Update.img
xxd -l 16 PRIMEGO-5.0.1-Update.img
```

The first bytes are `AZ01`. For the tested image, the rootfs `PARTL` header
starts at byte offset `8192336`, the compressed rootfs size is stored as a
little-endian 32-bit value there, the rootfs SHA-1 is at offset `8192384`, and
the xz stream itself starts at offset `8192404`.

Extract the rootfs xz stream and decompress it:

```sh
mkdir -p unpacked-img/JP11

rootfs_partl_offset=8192336
rootfs_xz_offset=8192404
rootfs_xz_size=$(od -An -tu4 -j "$rootfs_partl_offset" -N4 PRIMEGO-5.0.1-Update.img)
rootfs_xz_size=${rootfs_xz_size//[[:space:]]/}

dd if=PRIMEGO-5.0.1-Update.img \
  of=unpacked-img/JP11/rootfs.img.xz \
  bs=1 skip="$rootfs_xz_offset" count="$rootfs_xz_size" status=progress

xz -dk unpacked-img/JP11/rootfs.img.xz
```

Mount the rootfs writable. This is the same general idea as the repository's
`./mount.sh --write` flow, but for this rootfs-only test a direct loop mount was
sufficient:

```sh
sudo mkdir -p /mnt/primego-rootfs
sudo mount -o loop,rw unpacked-img/JP11/rootfs.img /mnt/primego-rootfs
```

Enable OpenSSH from the vendor systemd unit tree and add an SSH config drop-in.
Trying to enable it only under `/etc/systemd/system/multi-user.target.wants`
did not survive correctly on my upgraded device, likely because `/etc` is
overlaid from persistent storage.

```sh
sudo mkdir -p /mnt/primego-rootfs/usr/lib/systemd/system/multi-user.target.wants
sudo ln -s ../sshd.service \
  /mnt/primego-rootfs/usr/lib/systemd/system/multi-user.target.wants/sshd.service

sudo mkdir -p /mnt/primego-rootfs/etc/ssh/sshd_config.d
sudo tee /mnt/primego-rootfs/etc/ssh/sshd_config.d/10-az0x.conf >/dev/null <<'EOF'
HostKey /etc/ssh/ssh_host_ed25519_key
HostKeyAlgorithms ssh-ed25519-cert-v01@openssh.com,ssh-ed25519

PermitRootLogin yes
PasswordAuthentication yes
KbdInteractiveAuthentication no
EOF
```

Set a root password. The repository's older scripted examples use
`denonprime4`; for a real image, choose your own password.

```sh
sudo chroot /mnt/primego-rootfs passwd root
```

Unmount, recompress, and calculate the new hash:

```sh
sync
sudo umount /mnt/primego-rootfs

rm -f unpacked-img/JP11/rootfs.img.xz
xz -vk9eT0 --check=crc64 unpacked-img/JP11/rootfs.img
sha1sum unpacked-img/JP11/rootfs.img.xz
```

Rebuild the AZ01 image by keeping the stock header and splash/recovery payloads,
then replacing only the rootfs `PARTL` length, rootfs SHA-1, and rootfs xz
payload. The final 16 bytes are the same zero padding seen after the xz stream
in the tested image.

```sh
stock=PRIMEGO-5.0.1-Update.img
out=PRIMEGO-5.0.1-SSH-Update.img
rootxz=unpacked-img/JP11/rootfs.img.xz

rootfs_partl_offset=8192336
rootfs_partl_size_offset=8192336
rootfs_sha1_offset=8192384
rootfs_xz_offset=8192404

{
  head -c "$rootfs_partl_size_offset" "$stock"
  perl -e 'print pack("V", shift)' "$(stat -c%s "$rootxz")"
  dd if="$stock" bs=1 \
    skip=$((rootfs_partl_size_offset + 4)) \
    count=$((rootfs_sha1_offset - rootfs_partl_size_offset - 4)) \
    status=none
  sha1sum "$rootxz" | awk '{print $1}' | xxd -r -p
  cat "$rootxz"
  dd if=/dev/zero bs=1 count=16 status=none
} >"$out"
```

For my working v2 image, this produced:

```text
rootfs.img.xz size: 168550276 bytes
rootfs.img.xz SHA-1: 2df256c569fe7c098883436411ac50b175aad926
output image size: 176742696 bytes
```

The resulting image flashed successfully on the tested PRIME GO. After boot,
`systemctl status sshd` showed the service running, and SSH login as `root`
worked.

## Open Questions

- Exact AZ01 container structure and checksums/signatures.
- Minimal safe unpack/repack procedure for 5.x images.
- Whether PRIME 4, PRIME 4+, PRIME 2, SC5000/6000, and Mixstream variants share
  the same 5.x container details.
- Whether the `/etc` overlay behaviour differs between upgraded devices and
  freshly flashed devices.
