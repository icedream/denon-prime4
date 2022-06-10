SRATOM_VERSION = 0.6.10
SRATOM_SOURCE = sratom-$(SRATOM_VERSION).tar.bz2
SRATOM_SITE = https://download.drobilla.net
SRATOM_INSTALL_STAGING = YES
SRATOM_LICENSE = ISC

$(eval $(waf-package))
