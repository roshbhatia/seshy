#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_dir"
output_dir=${SESHY_MEDIA_OUTPUT_DIR:-"$repo_dir/docs"}
mkdir -p "$output_dir"

media_fingerprint() {
  {
    printf '%s\n' flake.lock flake.nix go.mod go.sum README.md hack/seshy.tape hack/screenshots.sh
    find cmd internal -type f -name '*.go' ! -name '*_test.go' -print | sort
  } | while IFS= read -r path; do
    sha256sum "$path"
  done | sha256sum | cut -d ' ' -f 1
}

media_is_valid() {
  [[ -s $output_dir/seshy.png && -s $output_dir/seshy.gif ]] || return 1
  [[ $(identify -format '%m' "$output_dir/seshy.png") == PNG ]]
  [[ $(identify -format '%m' "$output_dir/seshy.gif[0]") == GIF ]]
}

if [[ ${1:-} == "--check" ]]; then
  expected=$(media_fingerprint)
  current=$(cat "$output_dir/.seshy-media.sha256" 2> /dev/null || true)
  if [[ $current != "$expected" ]] || ! media_is_valid; then
    echo "Seshy media is stale; run ./hack/screenshots.sh" >&2
    exit 1
  fi
  exit 0
fi

media_root=$(mktemp -d)
trap 'rm -rf "$media_root"' EXIT
go_mod_cache=$(go env GOMODCACHE)
export HOME="$media_root/home"
export GOMODCACHE="$go_mod_cache"
export XDG_CONFIG_HOME="$media_root/config"
export XDG_STATE_HOME="$media_root/state"
mkdir -p "$HOME/src" "$XDG_CONFIG_HOME" "$XDG_STATE_HOME"
HOME=$(cd "$HOME" && pwd -P)
export HOME
export SESHY_SESSIONS_DIR="$HOME/sessions"
export SESHY_ARCHIVE_DIR="$HOME/archive"

for repository in auth-api policy-engine developer-portal; do
  path="$HOME/src/$repository"
  git init -q "$path"
  git -C "$path" config user.email demo@example.invalid
  git -C "$path" config user.name "Seshy Demo"
  printf '# %s\n' "$repository" > "$path/README.md"
  git -C "$path" add README.md
  git -C "$path" commit -qm initial
done

mkdir -p "$media_root/bin"
go build -o "$media_root/bin/sy" ./cmd/sy
export PATH="$media_root/bin:$PATH"
sy new auth-hardening "$HOME/src/auth-api" "$HOME/src/policy-engine" "$HOME/src/developer-portal" > /dev/null
mkdir -p "$HOME/sessions/release-readiness/api" "$HOME/archive/incident-retro/logs"

freeze --execute "sy status auth-hardening" \
  --output "$output_dir/seshy.png" \
  --width 1000 \
  --padding 24 \
  --margin 16 \
  --window

vhs "$repo_dir/hack/seshy.tape" --output "$output_dir/seshy.gif"
chmod 0644 "$output_dir/seshy.png" "$output_dir/seshy.gif"

if ! media_is_valid; then
  echo "Seshy media generation produced an invalid image" >&2
  exit 1
fi
media_fingerprint > "$output_dir/.seshy-media.sha256"
