LIBKEYFINDER_VERSION = 2.2.6
LIBKEYFINDER_SOURCE = libkeyfinder-$(LIBKEYFINDER_VERSION).tar.gz
LIBKEYFINDER_SITE = $(call github,mixxxdj,libkeyfinder,v$(LIBKEYFINDER_VERSION))
LIBKEYFINDER_INSTALL_STAGING = YES
LIBKEYFINDER_INSTALL_TARGET = YES
LIBKEYFINDER_LICENSE = GPLv3

ifeq ($(BR2_PACKAGE_FFTW_DOUBLE),y)
LIBKEYFINDER_DEPENDENCIES += fftw-double
endif

$(eval $(cmake-package))
