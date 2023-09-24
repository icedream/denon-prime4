#!/bin/sh

# which product are we running on?
APPNAME=`cat /sys/firmware/devicetree/base/inmusic,product-code`

# detect installation path of our custom scripts
scriptspath="$(command -v setup-prerequisites.sh)"
if [ -z "$scriptspath" ]; then
  scriptspath="$(pwd)"
else
  scriptspath="$(dirname "$scriptspath")"
fi

# set up cpu assignment and other device stuff
"$scriptspath"/setup-prerequisites.sh $APPNAME

# test for an existing bus daemon, just to be safe
if test -z "$DBUS_SESSION_BUS_ADDRESS" ; then
  # if not found, launch a new one
  eval `dbus-launch --sh-syntax`
fi

# set up screen/input rotation
source "$scriptspath"/setup-screenrotation.sh $APPNAME

# run upower if it is available
if command -v upower; then
  "$(dirname $(readlink -f $(command -v upower)))/../libexec/upowerd" &
fi

# run command in gdbserver if wanted
if [ -n "${GDBSERVER:-}" ]; then
  exec gdbserver :2345 mixxx "$@"
fi

exec mixxx "$@"
