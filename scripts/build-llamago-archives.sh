#!/usr/bin/env bash
#
# build-llamago-archives.sh
#
# Builds the llama.cpp static archives for the CURRENT OS/arch and installs them
# into third_party/llama-go/<os>/<arch>/, so the vendored llama-go can link on
# Linux and macOS. This is the bash analogue of the llama-go-windows skill's
# reference/build.ps1 (which does the same thing on Windows/MinGW).
#
# The vendored third_party/llama-go ships only Windows/MinGW archives and a
# headers-only llama.cpp/ tree, so the llama.cpp SOURCE is fetched here and
# compiled into the per-OS archives. The resulting .a files are committed
# (matching the committed Windows archives), so builds thereafter need only the
# compiler on PATH - not a llama.cpp rebuild.
#
# USAGE (run on the TARGET OS):
#   ./scripts/build-llamago-archives.sh            # auto-detect the ref
#   LLAMA_GO_REF=<commit-or-tag> ./scripts/build-llamago-archives.sh   # pin it
#
#   LLAMA_GO_REF  (optional) - the llama-go commit/tag whose wrapper.h and
#     cgo_headers are vendored in third_party/llama-go. When unset the script
#     auto-detects the ref by trying HEAD, then every tag (newest-first), and
#     picking the first whose wrapper.h matches the vendored copy. When set it
#     verifies the ref matches (ABI guard) and aborts if not. The right ref is
#     the one whose wrapper.h is byte-identical to third_party/llama-go/wrapper.h.
#
#   Run it once per OS/arch you need to support:
#     - Linux amd64  -> writes third_party/llama-go/linux/amd64/
#     - macOS arm64  -> writes third_party/llama-go/darwin/arm64/
#     - macOS amd64  -> writes third_party/llama-go/darwin/amd64/
#
# REQUIREMENTS
#   Linux : gcc/g++, cmake, make (or ninja), git, ar
#   macOS : Xcode Command Line Tools (clang, clang++, cmake, make, ar), git
#
# NOTES
#   - The vendored wrapper.cpp is compiled (not the upstream one) against the
#     vendored headers, so the libbinding.a ABI matches the Go files in
#     third_party/llama-go.
#   - Linux builds with OpenMP (GGML_OPENMP=ON): the resulting binary depends on
#     libgomp.so.1 at runtime (bundle it in the AppImage, declare libgomp1 in
#     the .deb).
#   - macOS builds with the Metal + BLAS backends; the required frameworks are
#     OS-provided, so nothing extra ships.
#
set -euo pipefail

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux|darwin) ;;
  *) echo "ERROR: unsupported OS '$OS' (expected linux or darwin)" >&2; exit 1 ;;
esac

# Allow cross-arch builds (e.g. building the x86_64 archives on an ARM64 CI
# runner). When GOARCH is unset, detect it from the host.
GOARCH="${GOARCH:-}"
if [ -z "$GOARCH" ]; then
  ARCH="$(uname -m)"
  case "$ARCH" in
    x86_64)         GOARCH="amd64" ;;
    arm64|aarch64)  GOARCH="arm64" ;;
    *) echo "ERROR: unsupported arch '$ARCH'" >&2; exit 1 ;;
  esac
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LLAMA_GO_ROOT="$REPO_ROOT/third_party/llama-go"
DEST="$LLAMA_GO_ROOT/$OS/$GOARCH"
LLAMA_GO_REF="${LLAMA_GO_REF:-}"
JOBS="${JOBS:-$(nproc 2>/dev/null || echo 4)}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> Cloning llama-go (sources only)"
git clone https://github.com/tcpipuk/llama-go "$TMP/llama-go"

# Determine the llama-go ref whose wrapper.h matches the vendored one. The
# Go<->C++ ABI depends on this being exactly right, so pick it by ABI guard
# rather than guessing. If LLAMA_GO_REF is set, verify it; otherwise
# auto-detect by trying HEAD, then all tags (newest-first), and taking the
# first ref whose wrapper.h matches the vendored copy.
find_matching_ref() {
  local refs=()
  refs+=("HEAD")
  while IFS= read -r t; do
    refs+=("$t")
  done < <(git -C "$TMP/llama-go" tag --sort=-version:refname)

  for ref in "${refs[@]}"; do
    if ! git -C "$TMP/llama-go" checkout --force "$ref" >/dev/null 2>&1; then
      continue
    fi
    if diff -q "$TMP/llama-go/wrapper.h" "$LLAMA_GO_ROOT/wrapper.h" >/dev/null 2>&1; then
      echo "$ref"
      return 0
    fi
  done
  return 1
}

if [ -n "$LLAMA_GO_REF" ]; then
  REF="$LLAMA_GO_REF"
  git -C "$TMP/llama-go" checkout --force "$REF" >/dev/null 2>&1 || {
    echo "ERROR: could not checkout llama-go '$LLAMA_GO_REF'" >&2
    exit 1
  }
  # ABI guard: the fetched source must match the vendored headers.
  if ! diff -q "$TMP/llama-go/wrapper.h" "$LLAMA_GO_ROOT/wrapper.h" >/dev/null 2>&1; then
    echo "ERROR: llama-go $LLAMA_GO_REF wrapper.h does not match the vendored" >&2
    echo "       third_party/llama-go/wrapper.h. Choose a different LLAMA_GO_REF." >&2
    exit 1
  fi
else
  REF="$(find_matching_ref)" || {
    echo "ERROR: could not find a llama-go ref whose wrapper.h matches the" >&2
    echo "       vendored third_party/llama-go/wrapper.h." >&2
    exit 1
  }
  echo "==> Auto-detected llama-go ref: $REF"
fi

echo "==> Updating llama.cpp submodule at $REF"
git -C "$TMP/llama-go" submodule update --init --recursive

CMAKE="${CMAKE:-cmake}"
CC="${CC:-gcc}"
CXX="${CXX:-g++}"
if [ "$OS" = "darwin" ]; then
  CC="${CC:-clang}"
  CXX="${CXX:-clang++}"
fi

BUILD="$TMP/build"
mkdir -p "$BUILD"

echo "==> CMake configure ($OS/$GOARCH)"
EXTRA=()
if [ "$OS" = "linux" ]; then
  EXTRA+=(-DGGML_OPENMP=ON)
  EXTRA+=(-DCMAKE_C_COMPILER="$CC" -DCMAKE_CXX_COMPILER="$CXX")
elif [ "$OS" = "darwin" ]; then
  # Map the Go arch name to the Apple arch name cmake expects.
  case "$GOARCH" in
    amd64) OSX_ARCH="x86_64" ;;
    arm64) OSX_ARCH="arm64" ;;
    *) echo "ERROR: unsupported GOARCH '$GOARCH' for darwin" >&2; exit 1 ;;
  esac
  EXTRA+=(-DGGML_METAL=ON -DGGML_BLAS=ON)
  EXTRA+=(-DCMAKE_C_COMPILER="$CC" -DCMAKE_CXX_COMPILER="$CXX")
  EXTRA+=(-DCMAKE_OSX_DEPLOYMENT_TARGET=12.0)
  EXTRA+=(-DCMAKE_OSX_ARCHITECTURES="$OSX_ARCH")
fi

"$CMAKE" -S "$TMP/llama-go/llama.cpp" -B "$BUILD" \
  -DBUILD_SHARED_LIBS=OFF -DLLAMA_CURL=OFF -DCMAKE_BUILD_TYPE=Release \
  -DGGML_NATIVE=OFF -DGGML_AVX512=OFF -DGGML_AVX512_VBMI=OFF -DGGML_AVX512_VNNI=OFF \
  "${EXTRA[@]}"

echo "==> Building ggml llama llama-common (a few minutes)"
"$CMAKE" --build "$BUILD" --target ggml llama llama-common --config Release -j "$JOBS"
# On Apple the ggml meta-target builds ggml-metal + ggml-blas; build explicitly
# in case they are separate targets (ignore failures if already built).
if [ "$OS" = "darwin" ]; then
  "$CMAKE" --build "$BUILD" --target ggml-metal ggml-blas --config Release -j "$JOBS" || true
fi

echo "==> Compiling the vendored wrapper.cpp -> libbinding.a"
INC=(
  "-I$LLAMA_GO_ROOT/llama.cpp" "-I$LLAMA_GO_ROOT"
  "-I$LLAMA_GO_ROOT/llama.cpp/include" "-I$LLAMA_GO_ROOT/llama.cpp/ggml/include"
  "-I$LLAMA_GO_ROOT/llama.cpp/common" "-I$LLAMA_GO_ROOT/llama.cpp/vendor"
  "-I$LLAMA_GO_ROOT/cgo_headers" "-I$LLAMA_GO_ROOT/cgo_headers/llama.cpp"
  "-I$LLAMA_GO_ROOT/cgo_headers/llama.cpp/include"
  "-I$LLAMA_GO_ROOT/cgo_headers/llama.cpp/ggml/include"
  "-I$LLAMA_GO_ROOT/cgo_headers/llama.cpp/common"
  "-I$LLAMA_GO_ROOT/cgo_headers/llama.cpp/thirdparty"
)
CXX_EXTRA=()
if [ "$OS" = "darwin" ]; then
  CXX_EXTRA+=(-mmacosx-version-min=12.0)
fi

# shellcheck disable=SC2086
"$CXX" "${INC[@]}" "${CXX_EXTRA[@]}" -O3 -DNDEBUG -std=c++17 -fPIC \
  -c "$LLAMA_GO_ROOT/wrapper.cpp" -o "$TMP/wrapper.o"
ar crs "$TMP/libbinding.a" "$TMP/wrapper.o"

echo "==> Installing archives to $DEST"

# Locate each built archive by name rather than hardcoding a path, because the
# cmake output name can be either "lib<base>.a" or "<base>.a" depending on the
# llama.cpp version/backend. Abort if an archive is missing so a build that did
# not produce a needed library fails loudly here instead of at `go build` link.
install_archive() {
  local base="$1" dest="$2"
  local src
  src="$(find "$BUILD" -name "lib${base}.a" -print -quit 2>/dev/null)"
  if [ -z "$src" ]; then
    src="$(find "$BUILD" -name "${base}.a" -print -quit 2>/dev/null)"
  fi
  if [ -z "$src" ] || [ ! -f "$src" ]; then
    echo "ERROR: could not find archive for '${base}' under $BUILD" >&2
    exit 1
  fi
  cp "$src" "$DEST/$dest"
}

mkdir -p "$DEST"
install_archive llama-common      libllama-common.a
install_archive llama-common-base libllama-common-base.a
install_archive llama             libllama.a
install_archive ggml              libggml.a
install_archive ggml-base         libggml-base.a
install_archive ggml-cpu          libggml-cpu.a
cp "$TMP/libbinding.a" "$DEST/libbinding.a"
if [ "$OS" = "darwin" ]; then
  install_archive ggml-metal libggml-metal.a
  install_archive ggml-blas  libggml-blas.a
fi

echo "==> Done. Installed archives:"
ls -1 "$DEST"
