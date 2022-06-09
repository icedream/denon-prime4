MIXXX_VERSION = 2.3.2
MIXXX_SOURCE = mixxx-$(MIXXX_VERSION).tar.gz
MIXXX_SITE = https://github.com/mixxxdj/mixxx/archive/refs/tags/$(MIXXX_VERSION).tar.gz
MIXXX_INSTALL_STAGING = NO
MIXXX_INSTALL_TARGET = YES
MIXXX_LICENSE = GPLv2
# MIXXX_CONF_OPTS = -DCMAKE_INSTALL_PREFIX=/usr
# MIXXX_DEPENDENCIES = \
# 	protobuf vamp-plugin-sdk rubberband soundtouch \
# 	libid3tag taglib \
# 	lame libogg libmad libvorbis libmp4v2 faad2 opusfile wavpack \
# 	libshout libsndfile portmidi portaudio \
# 	sqlite upower lilv libopenmpt-modplug \
# 	qt5-declarative qtkeychain-qt5 qt5-svg

# Dependency list put together from
# 1. https://github.com/mixxxdj/mixxx/wiki/Compiling-On-Linux#arch--derivatives
# 2. https://aur.archlinux.org/cgit/aur.git/tree/PKGBUILD?h=mixxx-git
MIXXX_DEPENDENCIES = \
	protobuf libsoundtouch \
	libid3tag taglib \
	libogg libmad mp4v2 faad2 opusfile \
	libshout libsndfile portaudio \
	sqlite upower \
	qt5 qt5declarative qt5x11extras \
	\
	libebur128

ifeq ($(BR2_PACKAGE_LAME),y)
MIXXX_DEPENDENCIES += lame
endif

ifeq ($(BR2_PACKAGE_LIBVORBIS),y)
MIXXX_DEPENDENCIES += libvorbis
endif

ifeq ($(BR2_PACKAGE_LIBTHEORA),y)
MIXXX_DEPENDENCIES += libtheora
endif

ifeq ($(BR2_PACKAGE_LIBEBUR128),y)
MIXXX_DEPENDENCIES += libebur128
endif

ifeq ($(BR2_PACKAGE_WAVPACK),y)
MIXXX_DEPENDENCIES += wavpack
endif

$(eval $(cmake-package))
