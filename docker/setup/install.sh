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
  mkdir "${dest}" -p
  tar -xvf "${SETUP_DIR}/sources/${tarball}" -C "${dest}"
  rm -rf "${SETUP_DIR}/sources/${tarball}"
}

# astyle.
download_and_extract "${ASTYLE_DOWNLOAD_URL}" "${SETUP_DIR}/astyle"
pushd "${SETUP_DIR}/astyle/astyle/build/gcc"
make -j"$(nproc)"
cp bin/astyle /usr/bin
popd

# yapf.
YAPF_DIR="/opt/yapf"
YAPF_GIT_URL="https://github.com/google/yapf.git"
mkdir "${YAPF_DIR}" -p
git clone "${YAPF_GIT_URL}" "${YAPF_DIR}"
echo -e "#!/usr/bin/env bash\nPYTHONPATH=\"${YAPF_DIR}\" python2 \"${YAPF_DIR}/yapf\" \"\${@}\"" > /usr/bin/indent-python2
echo -e "#!/usr/bin/env bash\nPYTHONPATH=\"${YAPF_DIR}\" python3 \"${YAPF_DIR}/yapf\" \"\${@}\"" > /usr/bin/indent-python3
chmod +x /usr/bin/indent-python*


# vim:set ts=2 sw=2 et:

