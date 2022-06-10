SORD_VERSION = 0.16.10
SORD_SOURCE = sord-$(SORD_VERSION).tar.bz2
SORD_SITE = https://download.drobilla.net
SORD_INSTALL_STAGING = YES
SORD_LICENSE = ISC

$(eval $(waf-package))
