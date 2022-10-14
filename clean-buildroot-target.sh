#!/bin/bash

set -e

(
    cd buildroot/*/

    # wipe output/target and trigger a reinstall of all buildroot packages
    # see https://stackoverflow.com/a/49862790
    rm -rf \
        output/images \
        output/target \
        output/staging \
        output/build/packages-file-list-staging.txt \
        output/build/packages-file-list.txt \
        output/build/build-time.log
    find output/build/ -mindepth 1 -maxdepth 1 -not -name 'host-*' -exec rm -rf {} \;
    find output/ \( \
        -name .stamp_installed \
        -name .stamp_configured \
        -name .stamp_built \
        -or -name .stamp_images_installed \
        -or -name .stamp_target_installed \
        -or -name .stamp_staging_installed \
        -or -name .root \
        -or -name '.br2-external*' \
        \) -and -not -path '*/host-*/*' -delete
    # rm -f output/build/host-gcc-final-*/.stamp_host_installed
)
