#!/bin/sh

# This script inspects the meson build configuration and identifies the
# appropriate path to install Python packages into. It's appended to the
# remaining arguments of this script. Those arguments are assumed to be a python
# pip install command.

set -xe

LIBDIR=$(${MESONINTROSPECT} --buildoptions ${MESON_BUILD_ROOT} | jq -r '.[]|select(.name=="libdir")|.value')
PROJECT_NAME=$(${MESONINTROSPECT} --projectinfo ${MESON_BUILD_ROOT} | jq -r '.descriptive_name')

# --target has to be passed as a command-line argument. pip appears to ignore
# the environment variable equivalent PIP_TARGET value.
$@ --target ${MESON_INSTALL_DESTDIR_PREFIX}/${LIBDIR}/${PROJECT_NAME}
