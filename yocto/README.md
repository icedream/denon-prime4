# Yocto/OpenEmbedded Integration for Denon PRIME 4

This directory contains the Yocto/OpenEmbedded layer (`meta-denon`) for building custom software for the Denon PRIME 4 (AZ01 format).

## Overview

**IMPORTANT:** This Yocto layer is for **5.x firmware only** (Yocto-based). Your PRIME 4 is running **4.x firmware (Buildroot-based)**, which uses the `buildroot-customizations/` directory instead.

The `meta-denon` Yocto layer provides:
- Recipe for building a complete Denon PRIME 4 5.x image
- Qt 6.7.2 packages (matching 5.x firmware)
- Audio, network, and USB gadget support
- Integration with `meta-rockchip` BSP layer

**For 4.x firmware (your device):** Use `buildroot-customizations/` instead of Yocto.

## Directory Structure

```
yocto/
├── README.md                    # This file
└── meta-denon/
    ├── conf/
    │   └── layer.conf           # Yocto layer configuration
    └── recipes-core/
        └── images/
            └── denon-prime4-image.bb  # Main image recipe
```

## Prerequisites

1. **Yocto Build Environment** (see [Yocto Setup Guide](YOCTO-SETUP-GUIDE.md))
2. **meta-rockchip layer** for Rockchip RK3288 BSP support
3. **Buildroot** (for 4.x firmware, separate from Yocto)

**Important:** The `meta-openembedded` repository already contains `meta-oe`, `meta-python`, `meta-networking`, and other layers as subdirectories. You do NOT need to clone them separately.

## Building with Yocto

### 1. Set Up Build Environment

```bash
# Clone Yocto layers (see Yocto Setup Guide)
mkdir ~/yocto-prime4
cd ~/yocto-prime4
git clone -b scarthgap https://git.yoctoproject.org/poky.git
git clone -b scarthgap https://git.openembedded.org/meta-openembedded
# Note: meta-oe, meta-python, meta-networking are already in meta-openembedded
git clone https://github.com/JeffyCN/meta-rockchip.git

# Initialize build
cd ~/yocto-prime4/poky
source oe-init-build-env build-prime4
```

### 2. Configure Layers

Edit `conf/bblayers.conf` to include all layers:

```bash
# conf/bblayers.conf

BBLAYERS ?= " \
  ${HOME}/yocto-prime4/poky/meta \
  ${HOME}/yocto-prime4/poky/meta-poky \
  ${HOME}/yocto-prime4/poky/meta-yocto-bsp \
  ${HOME}/yocto-prime4/meta-openembedded/meta-oe \
  ${HOME}/yocto-prime4/meta-openembedded/meta-python \
  ${HOME}/yocto-prime4/meta-openembedded/meta-networking \
  ${HOME}/yocto-prime4/meta-openembedded/meta-multimedia \
  ${HOME}/yocto-prime4/meta-rockchip \
  ${HOME}/yocto-prime4/denon-prime4/yocto/meta-denon \
"
```

**Note:** Layers from `meta-openembedded` are referenced as subdirectories.

### 3. Configure MACHINE

```bash
# In conf/local.conf
MACHINE = "rk3288-evb"  #或 custom az01 machine
```

### 4. Build

```bash
bitbake denon-prime4-image
```

## Integration with Buildroot (4.x)

The Yocto build system is **separate** from the Buildroot-based 4.x build system. They coexist in the repository:

```
denon-prime4/
├── buildroot-customizations/   # 4.x firmware (Buildroot)
├── yocto/                      # 5.x firmware (Yocto)
│   └── meta-denon/
├── tools/
│   ├── extract-firmware.sh     # Extract 5.x firmware
│   ├── repack-firmware.sh      # Repack 5.x firmware
│   └── calculate-hashes.sh     # Calculate partition hashes
└── docs/
    └── reveng/
        ├── yocto-build-guide.md
        └── firmware-modification-plan.md
```

**Key Points:**
- 4.x firmware uses Buildroot (existing `buildroot-customizations/`)
- 5.x firmware uses Yocto (new `yocto/meta-denon/`)
- Both build systems produce compatible AZ01 container format
- No changes to existing Buildroot build commands

## Firmware Modification Workflow

### For 5.x (Yocto):

1. **Build custom software with Yocto**
   ```bash
   bitbake denon-prime4-image
   ```

2. **Extract official firmware**
   ```bash
   ./tools/extract-firmware.sh PRIME4-5.0.0-Update.img /tmp/firmware-work
   ```

3. **Modify firmware filesystem**
   ```bash
   sudo mount -o loop,rw /tmp/firmware-work/firmware.ext2 /mnt/firmware
   # Make changes...
   sudo umount /mnt/firmware
   ```

4. **Repack firmware**
   ```bash
   ./tools/repack-firmware.sh /tmp/firmware-work/firmware.ext2 PRIME4-MODIFIED-5.0.0-Update.img
   ```

5. **Flash to USB**
   ```bash
   dd if=PRIME4-MODIFIED-5.0.0-Update.img of=/dev/sdX bs=1M
   ```

## Adding Custom Recipes

To add a custom application:

1. **Create recipe directory**
   ```bash
   mkdir -p yocto/meta-denon/recipes-core/my-app
   ```

2. **Create recipe file**
   ```bash
   cat > yocto/meta-denon/recipes-core/my-app/my-app_1.0.bb << 'EOF'
   SUMMARY = "My Custom App"
   LICENSE = "CLOSED"
   LIC_FILES_CHKSUM = "file://${COMMON_LICENSE_DIR}/MIT;md5=0835ade698e0bcf8506ecda2f7b4f302"
   
   SRC_URI = "file://my-app"
   
   do_install() {
       install -d ${D}${bindir}
       install -m 0755 my-app ${D}${bindir}/
   }
   
   FILES:${PN} = "${bindir}/*"
   EOF
   ```

3. **Add source file**
   ```bash
   mkdir -p yocto/meta-denon/recipes-core/my-app/files
   cp /path/to/your/app yocto/meta-denon/recipes-core/my-app/files/
   ```

4. **Build**
   ```bash
   bitbake my-app
   ```

## Troubleshooting

### Qt 6 Not Building
- Ensure Yocto 4.0/Kirkstone or newer
- Check `qt6-base` recipe exists in layers
- May need to add `qt6-base` to `DISTRO_FEATURES`

### RK3288 Not Supported
- Use similar Rockchip machine (rk3288-evb, px3-evb)
- Override kernel and U-Boot sources in local.conf
- Create custom machine configuration

### XZ Stream Issues
- Use `xz -9e` for proper footer
- Verify with `xz -t firmware.ext2.xz`
- Use `--single-stream` for incomplete streams

## Related Documentation

- [Yocto Build Guide](../docs/reveng/yocto-build-guide.md)
- [Firmware Image Structure](../docs/reveng/firmware-image-structure.md)
- [Firmware Modification Plan](../docs/reveng/firmware-modification-plan.md)
- [SSH Persistence Guide](../docs/reveng/ssh-persistence-guide.md)

## Limitations

1. **Signing Keys:** AZ01 format uses SHA1 hashes (not cryptographic signatures), but partition table must be updated correctly
2. **Kernel/U-Boot:** Modifying boot components requires matching signatures
3. **Hardware Specifics:** RK3288 support in public layers may be limited
4. **Qt Version:** Must match Qt 6.7.2 used in official firmware

## Next Steps

1. Test with `core-image-minimal` first
2. Add Qt 6 support
3. Create custom application recipe
4. Test on hardware (if available)
5. Document any RK3288-specific patches needed
