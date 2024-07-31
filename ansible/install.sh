#!/bin/sh

set -xe

DATADIR=$(${MESONINTROSPECT} --buildoptions ${MESON_BUILD_ROOT} | jq -r '.[]|select(.name=="datadir")|.value')
PROJECT_NAME=$(${MESONINTROSPECT} --projectinfo ${MESON_BUILD_ROOT} | jq -r '.descriptive_name')
COLLECTIONS_PATH=${MESON_INSTALL_DESTDIR_PREFIX}/${DATADIR}/${PROJECT_NAME}/ansible/collections

install --directory --mode 0755 ${COLLECTIONS_PATH}

ansible-galaxy collection install \
    --offline \
    --collections-path ${COLLECTIONS_PATH} \
    $@
