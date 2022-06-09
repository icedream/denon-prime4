PORTMIDI_VERSION = 2.0.3
PORTMIDI_SOURCE = portmidi-$(PORTMIDI_VERSION).tar.gz
# PORTMIDI_SITE = https://github.com/acoustid/chromaprint/releases/download/v$(PORTMIDI_VERSION)/chromaprint-$(PORTMIDI_VERSION).tar.gz
PORTMIDI_SITE = $(call github,PortMidi,portmidi,v$(PORTMIDI_VERSION))
PORTMIDI_INSTALL_STAGING = YES
PORTMIDI_INSTALL_TARGET = YES
PORTMIDI_LICENSE = Expat
# PORTMIDI_DEPENDENCIES = 

$(eval $(cmake-package))
