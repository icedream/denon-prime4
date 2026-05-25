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

## Open Questions

- Exact AZ01 container structure and checksums/signatures.
- Minimal safe unpack/repack procedure for 5.x images.
- Whether PRIME 4, PRIME 4+, PRIME 2, SC5000/6000, and Mixstream variants share
  the same 5.x container details.
- Whether the `/etc` overlay behaviour differs between upgraded devices and
  freshly flashed devices.
