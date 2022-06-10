LV2_VERSION = 1.18.4
LV2_SOURCE = lv2-$(LV2_VERSION).tar.bz2
LV2_SITE = https://lv2plug.in/spec
LV2_INSTALL_STAGING = YES
LV2_LICENSE = ISC

$(eval $(waf-package))
