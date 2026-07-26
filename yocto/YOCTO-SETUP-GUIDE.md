# Yocto Build Environment Setup Guide

This guide explains how to set up the Yocto build environment for the Denon PRIME 4, including how to handle the meta-openembedded structure and override kernel/U-Boot sources.

## Meta-OpenEmbedded Structure

The `meta-openembedded` repository already contains the following layers as subdirectories:
- `meta-oe` - OpenEmbedded extras
- `meta-python` - Python recipes
- `meta-networking` - Networking recipes
- `meta-multimedia` - Multimedia recipes
- `meta-filesystems` - Filesystem recipes
- And more...

**You do NOT need to clone these separately!** They are already part of the `meta-openembedded` repository.

## Setup Steps

### 1. Clone Required Layers

```bash
cd ~/yocto-prime4

# Clone Poky (Yocto Project)
git clone -b scarthgap https://git.yoctoproject.org/poky.git

# Clone meta-openembedded (contains meta-oe, meta-python, meta-networking, etc.)
git clone -b scarthgap https://git.openembedded.org/meta-openembedded

# Clone meta-rockchip (Rockchip BSP)
git clone https://github.com/JeffyCN/meta-rockchip.git
```

### 2. Initialize Build Environment

```bash
cd ~/yocto-prime4/poky
source oe-init-build-env build-prime4
```

### 3. Configure Layers

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

**Note:** The layers from `meta-openembedded` are referenced as subdirectories (e.g., `${HOME}/yocto-prime4/meta-openembedded/meta-oe`).

### 4. Configure Machine

Edit `conf/local.conf`:

```bash
# conf/local.conf

# Machine configuration
MACHINE = "az01"

# Skip patch checks (common for embedded)
WARN_QA:remove = "patch-fuzz"
ERROR_QA:remove = "patch-status"

# Inherit Rockchip image class
INHERIT:append = " rockchip-image"

# Package management
PACKAGE_CLASSES = "package_ipk"

# Parallelism
BB_NUMBER_THREADS = "8"
PARALLEL_MAKE = "-j 8"
```

## Firmware Version Clarification

**IMPORTANT:** There are TWO different firmware versions for the PRIME 4:

### 4.x Firmware (Buildroot)
- **Build system:** Buildroot
- **Build directory:** `buildroot-customizations/`
- Use this if your physical device is running 4.x firmware (check with `uname -a` over SSH - a Buildroot kernel version string looks like `6.1.111-inmusic-2024-09-19-rt41`).

### 5.x Firmware (Yocto - what this directory builds)
- **BUILD_TAG:** `jenkins-Planck-Embedded_Yocto_Branch-472`
- **Build system:** Yocto, "Scarthgap" (5.0 LTS) release line
- **Kernel:** exactly reconstructed as Linux **6.6.119** + official RT patch **6.6.119-rt67**
- **Build directory:** `yocto/meta-denon/`

## Kernel: use the reconstructed recipe, not meta-rockchip's linux-rockchip

The exact kernel version, RT patch, `.config`, and board DTS have already been
identified, verified, and vendored into this repo. **Do not** try to override
`meta-rockchip`'s `linux-rockchip` recipe (it tops out at Linux 6.1 and is a
vendor BSP fork, not a match for the mainline-style 5.x kernel). Instead the
machine conf (`conf/machine/az01.conf`) already points
`PREFERRED_PROVIDER_virtual/kernel` at `linux-denon-az01`, defined in
`recipes-kernel/linux/linux-denon-az01_6.6.119.bb`.

See **[`KERNEL-SETUP-GUIDE.md`](KERNEL-SETUP-GUIDE.md)** for the full
step-by-step (fetching the exact kernel.org tarball + RT patch, checksums,
GPG verification, and how the extracted `.config`/DTS were obtained) and
**[`docs/reveng/kernel-reconstruction-5.x.md`](../docs/reveng/kernel-reconstruction-5.x.md)**
for the full research trail, including which InMusic-proprietary drivers
(audio codec, power/fan management) cannot be reconstructed from public
sources.

Before building, make sure the layers are on the `scarthgap` branch (matches
the firmware's `arm-poky-linux-gnueabi-gcc 13.4.0` / Binutils `2.42` toolchain):

```bash
cd _yocto_prime4/poky              && git checkout scarthgap
cd ../meta-openembedded            && git checkout scarthgap
cd ../meta-rockchip                && git checkout scarthgap
```

## Building

Once configured, build the image:

```bash
# Build core image
bitbake core-image-minimal

# Or build with Qt 6
bitbake denon-prime4-image

# Build hello app
bitbake hello-prime4
```

## Troubleshooting

### RK3288 Not Found in meta-rockchip

If `meta-rockchip` doesn't have RK3288 support:

1. **Use a similar machine:**
   ```bash
   MACHINE = "px3-evb"  # or another RK3288 board
   ```

2. **Create custom machine config:**
   ```bash
   # In meta-denon/conf/machine/az01.conf
   inherit rockchip
   
   KBUILD_DEFCONFIG = "rk3288_linux_defconfig"
   KERNEL_DEVICETREE = "rk3288-az01-jc11.dtb"
   UBOOT_MACHINE = "rk3288_evb_defconfig"
   ```

3. **Override in local.conf:**
   ```bash
   PREFERRED_VERSION_linux-rockchip = "6.6%"
   SRC_URI:linux-rockchip = "git://github.com/rockchip-linux/kernel.git;protocol=https;branch=develop-6.1"
   ```

### Kernel Build Fails

1. **Check kernel version:**
   ```bash
   bitbake -s | grep linux-rockchip
   ```

2. **Use a known working kernel:**
   ```bash
   PREFERRED_VERSION_linux-rockchip = "6.1%"
   ```

3. **Add missing patches:**
   ```bash
   # Create patch directory
   mkdir -p yocto/meta-denon/recipes-core/linux/linux-rockchip/
   
   # Add patches
   FILESEXTRAPATHS:prepend := "${HOME}/yocto-prime4/denon-prime4/yocto/meta-denon/recipes-core/linux/linux-rockchip/:"
   ```

## Next Steps

1. **Test with core-image-minimal** to verify the build works
2. **Add Qt 6 support** for the PRIME 4
3. **Build hello-prime4** to test the meta-denon layer
4. **Customize kernel** with PRIME 4-specific drivers

## Resources

- [Yocto Project Documentation](https://docs.yoctoproject.org/)
- [meta-rockchip Layer](https://github.com/JeffyCN/meta-rockchip)
- [OpenEmbedded Layer Index](https://layers.openembedded.org/)
- [Rockchip Developer Wiki](http://opensource.rock-chips.com/wiki)
