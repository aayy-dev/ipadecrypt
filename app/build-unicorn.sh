#!/bin/sh
# Build a static, iOS-patched Unicorn for appstore-helper and print the path
# of libunicorn.a. App Store sign-in needs it: ipatool signs sign-in requests
# by running Apple's x86_64 FairPlay code in Unicorn, which purego finds
# through dlsym in the helper itself (so it must be force-loaded and its
# symbols exported). This mirrors majd/ipatool's tools/build-ios.sh.
#
# The result is cached in app/.build; delete that to rebuild.

set -eu
cd "$(dirname "$0")"

# Keep in sync with ipatool's internal/sap/unicorn/artifact.go.
VERSION=2.1.4
SHA256=ea8863f095a0136388694e5a6063afd9bb7650e30243dd6251af59c5ce5601f4

OUT="$PWD/.build/unicorn-$VERSION-ios"
LIB="$OUT/build/libunicorn.a"

if [ ! -f "$LIB" ]; then
  for tool in cmake curl shasum xcrun; do
    command -v "$tool" >/dev/null 2>&1 || { echo "$tool is required to build Unicorn" >&2; exit 1; }
  done

  rm -rf "$OUT"
  mkdir -p "$OUT"
  curl -fsSL --retry 3 "https://github.com/unicorn-engine/unicorn/archive/refs/tags/$VERSION.tar.gz" -o "$OUT/unicorn.tar.gz"
  echo "$SHA256  $OUT/unicorn.tar.gz" | shasum -a 256 -c - >&2
  tar -xzf "$OUT/unicorn.tar.gz" -C "$OUT"
  patch -s -d "$OUT/unicorn-$VERSION" -p1 < patches/unicorn-ios.patch

  SDKROOT=$(xcrun --sdk iphoneos --show-sdk-path)
  ARCHFLAGS='-arch arm64' cmake -S "$OUT/unicorn-$VERSION" -B "$OUT/build" \
    -DCMAKE_SYSTEM_NAME=iOS -DCMAKE_OSX_SYSROOT="$SDKROOT" \
    -DCMAKE_OSX_ARCHITECTURES=arm64 -DCMAKE_OSX_DEPLOYMENT_TARGET=15.0 \
    -DCMAKE_C_FLAGS='-target arm64-apple-ios15.0' -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_SHARED_LIBS=OFF -DUNICORN_ARCH=x86 -DUNICORN_BUILD_TESTS=OFF -DUNICORN_INSTALL=OFF >&2
  cmake --build "$OUT/build" --parallel "$(sysctl -n hw.ncpu)" >&2
fi

echo "$LIB"
