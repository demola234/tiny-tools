#!/bin/sh
set -eu

repo="${MOCKMACHINA_REPO:-demola234/tiny-tools}"
api="${MOCKMACHINA_API:-https://api.github.com/repos/$repo/releases?per_page=100}"
base="${MOCKMACHINA_DOWNLOAD_BASE:-https://github.com/$repo/releases/download}"
version="${MOCKMACHINA_VERSION:-}"
dest="${MOCKMACHINA_INSTALL_DIR:-}"

fail() {
  echo "mockmachina install: $*" >&2
  exit 1
}

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "this script is for macOS and Linux; on Windows use Scoop or the zip from the releases page" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "no build for $(uname -m)" ;;
esac

command -v curl >/dev/null 2>&1 || fail "curl is needed"

if [ -z "$version" ]; then
  version="$(curl -fsSL "$api" | grep -o '"tag_name": *"mock_machina/v[^"]*"' | head -n 1 | sed 's/.*mock_machina\///; s/"$//')"
  [ -n "$version" ] || fail "couldn't find a release; set MOCKMACHINA_VERSION"
fi

if [ -z "$dest" ]; then
  if [ -w /usr/local/bin ]; then
    dest=/usr/local/bin
  else
    dest="$HOME/.local/bin"
  fi
fi

archive="mockmachina_${version#v}_${os}_${arch}.tar.gz"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

curl -fsSL "$base/mock_machina/$version/$archive" -o "$work/$archive" || fail "couldn't download $archive"
curl -fsSL "$base/mock_machina/$version/checksums.txt" -o "$work/checksums.txt" || fail "couldn't download checksums.txt"

want="$(grep " $archive\$" "$work/checksums.txt" | cut -d ' ' -f 1)"
if command -v sha256sum >/dev/null 2>&1; then
  got="$(sha256sum "$work/$archive" | cut -d ' ' -f 1)"
else
  got="$(shasum -a 256 "$work/$archive" | cut -d ' ' -f 1)"
fi
[ -n "$want" ] && [ "$want" = "$got" ] || fail "checksum doesn't match for $archive; not installing"

tar -xzf "$work/$archive" -C "$work" mockmachina
mkdir -p "$dest"
install -m 755 "$work/mockmachina" "$dest/mockmachina"
echo "installed mockmachina $version to $dest/mockmachina"
case ":$PATH:" in
  *":$dest:"*) ;;
  *) echo "add $dest to your PATH to run it from anywhere" ;;
esac
