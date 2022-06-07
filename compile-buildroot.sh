#!/bin/bash -e
packages=(
  #busybox
  cairo
  chocolate-doom
  #dbus
  #dbus-glib
  doom-wad
  elfutils
  #expat
  #fbset
  fontconfig
  freetype
  glibc
  #host-gcc-final
  jpeg-turbo
  #kmod
  #libcap
  libdrm
  libevdev
  libffi
  libglib2
  libinput
  #libopenssl
  libpciaccess
  libpng
  libpthread-stubs
  #libsamplerate
  libxcb
  libxkbcommon
  libxml2
  #libzlib
  #linux
  lsof
  ltrace
  #mali-t76x # NOTE - wrong mali files? with this even Engine fails to start due to incompatible GPU
  matchbox
  matchbox-common
  matchbox-desktop
  matchbox-fakekey
  matchbox-keyboard
  matchbox-lib
  matchbox-panel
  mcookie
  mesa3d-demos
  mtdev
  nano
  ncurses
  openssh
  #openssl
  pcre
  pixman
  qt5
  sdl2
  sdl2_mixer
  sdl2_net
  #skeleton-init-common
  #skeleton-init-systemd
  strace
  #systemd
  #toolchain
  #tzdata
  util-linux
  util-linux-libs
  wayland
  weston
  xapp_xkbcomp
  xapp_xrandr
  xapp_xset
  xapp_xsetroot
  xcb-proto
  xdata_xbitmaps
  xdriver_xf86-input-keyboard
  xdriver_xf86-input-libinput
  xdriver_xf86-video-fbdev
  xdriver_xf86-video-vesa
  xfont_encodings
  xfont_font-alias
  xfont_font-cursor-misc
  xfont_font-misc-misc
  xkeyboard-config
  xlib_libICE
  xlib_libSM
  xlib_libX11
  xlib_libXau
  xlib_libXcursor
  xlib_libXdamage
  xlib_libXdmcp
  xlib_libXext
  xlib_libXfixes
  xlib_libXfont2
  xlib_libXft
  xlib_libXi
  xlib_libXinerama
  xlib_libXrandr
  xlib_libXrender
  xlib_libXres
  xlib_libXtst
  xlib_libXxf86vm
  xlib_libfontenc
  xlib_libxkbfile
  xlib_libxshmfence
  xlib_xtrans
  xserver_xorg-server
  #zlib
)

filter_package_files() {
  filter_str=''
  for package in "${packages[@]}"; do
    if [ -n "$filter_str" ]; then
      filter_str="$filter_str"'\|'
    fi
    filter_str="$filter_str"'^'"$package,"
  done

  grep "$filter_str" | tr ',' ' ' | awk '{print $2}'
}

# remove spaces since buildroot does not like that
export PATH="${PATH// /}"

./clone-buildroot.sh
make -C buildroot/*/ -j$(nproc)
tar -c -v -C buildroot/*/output/target/ --owner=root --group=root \
	$(cat buildroot/*/output/build/packages-file-list.txt | filter_package_files) \
	| \
sudo ./mount.sh --write tar -xp
sudo ./mount.sh --write systemctl enable sshd
if ! sudo ./mount.sh grep -q sshd /etc/group; then
  sudo ./mount.sh --write /sbin/addgroup -S sshd
fi
if ! sudo ./mount.sh grep -q sshd /etc/passwd; then
  sudo ./mount.sh --write /sbin/adduser -H -S -D -G sshd -h /var/empty sshd
fi
sudo ./mount.sh --write sed -i 's,#PermitRootLogin .\+,PermitRootLogin yes,g' /etc/ssh/sshd_config
(echo denonprime4 && echo denonprime4) | sudo ./mount.sh --write passwd root
sudo ./mount.sh --write mkdir -p /var/empty
