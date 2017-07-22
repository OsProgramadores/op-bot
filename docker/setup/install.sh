#!/usr/bin/env bash
set -e

ASTYLE_DOWNLOAD_URL="https://gigenet.dl.sourceforge.net/project/astyle/astyle/astyle%20${ASTYLE_VER}/astyle_${ASTYLE_VER}_linux.tar.gz"

download_and_extract() {
  src=${1}
  dest=${2}
  tarball=$(basename "${src}")

  if [ ! -f "${SETUP_DIR}/sources/${tarball}" ]; then
    echo "Downloading ${tarball}..."
    mkdir -p "${SETUP_DIR}/sources/"
    wget "${src}" -O "${SETUP_DIR}/sources/${tarball}"
  fi

  echo "Extracting ${tarball}..."
  mkdir "${dest}"
  tar -zxf "${SETUP_DIR}/sources/${tarball}" --strip=1 -C "${dest}"
  rm -rf "${SETUP_DIR}/sources/${tarball}"
}

download_and_extract "${ASTYLE_DOWNLOAD_URL}" "${SETUP_DIR}/astyle"
pushd "${SETUP_DIR}/astyle/build/gcc"
make -j"$(nproc)"
cd bin
ln -s "$(pwd)/astyle" /usr/bin/astyle
popd

# vim:set ts=2 sw=2 et:

