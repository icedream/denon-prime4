#!/bin/sh

APPNAME=$1

panelRotationFile=/sys/firmware/devicetree/base/inmusic,panel-rotation
touchRotationFile=/sys/firmware/devicetree/base/inmusic,touch-rotation

[ -f $panelRotationFile ] && panelHex=`xxd -p -s2 -c2 $panelRotationFile` && panelDec=$((0x$panelHex))
[ -f $touchRotationFile ] && touchHex=`xxd -p -s2 -c2 $touchRotationFile` && touchDec=$((0x$touchHex))

if [ -n "$panelHex" ] && [ $(($panelDec % 90)) -eq 0 ]
then
	[ $panelDec -gt 180 ] && let panelDec=panelDec-360

	export AIR_SCREEN_ROTATION=$panelDec
	echo Screen rotation is set to $panelDec

	if [ -n "$touchHex" ] && [ $(($touchDec % 90)) -eq 0 ]
	then
		[ $touchDec -gt 180 ] && let touchDec=touchDec-360

		let rotation=panelDec-touchDec
		export QT_QPA_EVDEV_TOUCHSCREEN_PARAMETERS=rotate=$rotation
		echo Touch rotation is set to $rotation
	fi
else
	echo Screen rotation has not been set
fi

