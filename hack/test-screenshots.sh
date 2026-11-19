#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "${fixture:?}"' EXIT

copy_media() {
  cp "$repo_dir/docs/seshy.png" "$fixture/seshy.png"
  cp "$repo_dir/docs/seshy.gif" "$fixture/seshy.gif"
  chmod u+w "$fixture/seshy.png" "$fixture/seshy.gif"
}

refresh() {
  SESHY_MEDIA_OUTPUT_DIR="$fixture" bash "$repo_dir/hack/screenshots.sh" "$@"
}

expect_rejection() {
  printf 'preserved\n' > "$fixture/.seshy-media.sha256"
  if refresh --refresh-fingerprint; then
    echo "ERROR: refresh accepted invalid media" >&2
    exit 1
  fi
  if [[ $(cat "$fixture/.seshy-media.sha256") != preserved ]]; then
    echo "ERROR: rejected media changed the fingerprint" >&2
    exit 1
  fi
}

copy_media
refresh --refresh-fingerprint
refresh --check

rm "$fixture/seshy.png"
expect_rejection

copy_media
cp "$fixture/seshy.gif" "$fixture/seshy.png"
expect_rejection

copy_media
cp "$fixture/seshy.png" "$fixture/seshy.gif"
expect_rejection
