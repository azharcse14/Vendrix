#!/usr/bin/env bash
# Tailwind v3.4 standalone CLI (Node chhara, 10.10): pin kora version ar sha256 mile tobei chalay.
# Binary website/.dart_tool e (gitignore). Udahoron: bash scripts/tailwind.sh --minify, bash scripts/tailwind.sh --watch
set -euo pipefail
VERSION=v3.4.19
case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) asset=tailwindcss-macos-arm64 sum=7fdeb00818b6214a337383063282b2361ecb08bbc08f8c8a7ba97ee1e2eaa4fe ;;
  Darwin-x86_64) asset=tailwindcss-macos-x64 sum=a597f407e0f1f03535731f5b42f1576a8152cb5fffc2f38e754722bc0c280045 ;;
  Linux-x86_64) asset=tailwindcss-linux-x64 sum=4af3198c015616ea7d6617974ec3d70d987ecc00c1ca8463b0a30fd65cc7c06e ;;
  Linux-aarch64 | Linux-arm64) asset=tailwindcss-linux-arm64 sum=e5b2d27694daa80cc52ec29553ba2c6bd43d86bd51a9d633ed24058b9c05a676 ;;
  *) echo "tailwind.sh: ei platform er binary pin nai: $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac
cd "$(dirname "$0")/../website"
bin=".dart_tool/tailwindcss-$VERSION"
if [ ! -x "$bin" ]; then
  mkdir -p .dart_tool
  curl -fsSL -o "$bin.tmp" "https://github.com/tailwindlabs/tailwindcss/releases/download/$VERSION/$asset"
  got=$( (command -v sha256sum >/dev/null && sha256sum "$bin.tmp" || shasum -a 256 "$bin.tmp") | cut -d' ' -f1)
  if [ "$got" != "$sum" ]; then
    rm -f "$bin.tmp"
    echo "tailwind.sh: sha256 mile na ($got)" >&2
    exit 1
  fi
  chmod +x "$bin.tmp" && mv "$bin.tmp" "$bin"
fi
exec "$bin" -c tailwind.config.js -i tailwind.css -o web/styles.css "$@"
