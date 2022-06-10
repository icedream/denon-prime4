LILV_VERSION = 0.24.14
LILV_SOURCE = lilv-$(LILV_VERSION).tar.bz2
LILV_SITE = https://download.drobilla.net
LILV_INSTALL_STAGING = YES
LILV_LICENSE = ISC
LILV_DEPENDENCIES = lv2 serd sord sratom

$(eval $(waf-package))
