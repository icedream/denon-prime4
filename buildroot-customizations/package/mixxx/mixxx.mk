MIXXX_VERSION = 2.3.2
MIXXX_SOURCE = mixxx-$(MIXXX_VERSION).tar.gz
MIXXX_SITE = https://github.com/mixxxdj/mixxx/archive/refs/tags/$(MIXXX_VERSION).tar.gz
MIXXX_INSTALL_STAGING = NO
MIXXX_INSTALL_TARGET = YES
# MIXXX_CONF_OPTS = -DCMAKE_INSTALL_PREFIX=/usr
# MIXXX_DEPENDENCIES = \
# 	protobuf vamp-plugin-sdk rubberband soundtouch \
# 	libid3tag taglib \
# 	lame libogg libmad libvorbis libmp4v2 faad2 opusfile wavpack \
# 	libshout libsndfile portmidi portaudio \
# 	sqlite upower lilv libopenmpt-modplug \
# 	qt5-declarative qtkeychain-qt5 qt5-svg
MIXXX_DEPENDENCIES = \
	protobuf libsoundtouch \
	libid3tag taglib \
	lame libogg libmad libvorbis mp4v2 faad2 opusfile wavpack \
	libshout libsndfile portaudio \
	sqlite upower \
	qt5

$(eval $(cmake-package))