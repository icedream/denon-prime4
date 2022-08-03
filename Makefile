# Settings

ENGINEOS_DEVICE=prime4
ENGINEOS_VENDOR=denon

export ENGINEOS_DEVICE
export ENGINEOS_VENDOR

# Scripts

SCRIPTS=\
	clean-buildroot-target\
	clone-buildroot\
	compile-buildroot\
	configure-buildroot\
	dist\
	generate-package-ignorelist\
	generate-updater-win\
	pack\
	unpack-updater\
	unpack

.PHONY: $(SCRIPTS)
$(SCRIPTS):
	./$@.sh

# Specific rules

unpacked-img/%.img.xz: unpacked-img/%.img
	xz -vk9eT0 --check=crc64 $<

unpacked-img/%.img.xz.sha1: unpacked-img/%.img.xz
	sha1sum $< | awk '{print $$1}' | xxd -r -p >$@

%.dtb: %.dts
	mkimage -f $< $@
