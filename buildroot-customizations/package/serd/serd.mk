SERD_VERSION = 0.30.16
SERD_SOURCE = serd-$(SERD_VERSION).tar.bz2
SERD_SITE = https://download.drobilla.net
SERD_INSTALL_STAGING = YES
SERD_LICENSE = ISC

$(eval $(waf-package))
