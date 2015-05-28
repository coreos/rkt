#!/bin/bash -e

# Derive a minimal base tree for hosting systemd from a cached coreos release pxe image

shopt -s nullglob

CACHED_IMG="${ITMP}/pxe.img"
ROOTFS="${ITMP}/rootfs"
FILELIST="${ITMP}/manifest.txt"
USR="${ITMP}/rootfs/usr"
SQUASH="${ITMP}/usr.squashfs"

# always start with an empty rootfs
[ -e "${ROOTFS}" ] && rm -Rf "${ROOTFS}"

mkdir -p "${ROOTFS}"

# create consolidated file list
cat manifest.d/* | sort -u > "${FILELIST}"


# derive $SQUASH from $CACHED_IMG

# AFAIK, that's the only way to make cpio to output to specified dir
pushd ${ITMP}
gzip -cd "${CACHED_IMG}" | cpio --unconditional --extract "usr.squashfs"
popd
unsquashfs -d "${USR}" -ef "${FILELIST}" "${SQUASH}"

# just leave the desired tree @ ${ROOTFS}
