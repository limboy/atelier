#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "Building a DMG requires macOS." >&2
  exit 1
fi

cd "$(dirname "${BASH_SOURCE[0]}")/.."

if [[ "${GITHUB_REF_TYPE:-}" == tag && "${GITHUB_REF_NAME:-}" == v* ]]; then
  if [[ -z "${APPLE_SIGNING_IDENTITY:-}" || -z "${APPLE_NOTARY_PROFILE:-}" ]]; then
    echo "Release builds require Apple signing and notarization credentials." >&2
    exit 1
  fi
fi

# MyGo uses the Developer ID and notary profile from the environment when set,
# and falls back to ad hoc signing for local builds and pull requests.
bun run build --platform darwin/universal

shopt -s nullglob
dmgs=(build/darwin-universal/*.dmg)
if [[ ${#dmgs[@]} -ne 1 ]]; then
  echo "Expected one DMG in build/darwin-universal, found ${#dmgs[@]}." >&2
  exit 1
fi
dmg="${dmgs[0]}"
if [[ -n "${APPLE_NOTARY_PROFILE:-}" ]]; then
  xcrun stapler validate "$dmg"
  spctl --assess --type open --context context:primary-signature --verbose=2 "$dmg"
fi
hdiutil verify "$dmg"

# Verify the copy inside the image, including the drag-to-Applications link.
mount_dir="$(mktemp -d)"
mounted=false
cleanup() {
  if [[ "$mounted" == true ]]; then
    hdiutil detach "$mount_dir" -quiet || hdiutil detach "$mount_dir" -force -quiet
  fi
  rmdir "$mount_dir"
}
trap cleanup EXIT

hdiutil attach "$dmg" -readonly -nobrowse -noautoopen -mountpoint "$mount_dir"
mounted=true

app="$mount_dir/Atelier.app"
plutil -lint "$app/Contents/Info.plist"
executable="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$app/Contents/Info.plist")"
binary="$app/Contents/MacOS/$executable"
test -x "$binary"
for arch in arm64 x86_64; do
  lipo "$binary" -verify_arch "$arch"
done
codesign --verify --deep --strict --all-architectures --verbose=2 "$app"
test "$(readlink "$mount_dir/Applications")" = /Applications

printf '\nVerified universal DMG: %s\n' "$dmg"
