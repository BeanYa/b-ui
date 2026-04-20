#!/usr/bin/env bash

set -euo pipefail

required_dirs=(
  "src"
  "src/backend"
  "src/frontend"
  "scripts"
  "scripts/ci"
  "packaging"
  "build"
  "build/out"
  "build/tmp"
  "build/reports"
  "dist"
  "dist/release"
)

required_files=(
  "build/out/.gitkeep"
  "build/tmp/.gitkeep"
  "build/reports/.gitkeep"
  "dist/release/.gitkeep"
  "scripts/ci/check-layout.sh"
)

missing=0

for path in "${required_dirs[@]}"; do
  if [[ ! -d "$path" ]]; then
    printf 'missing directory: %s\n' "$path" >&2
    missing=1
  fi
done

for path in "${required_files[@]}"; do
  if [[ ! -f "$path" ]]; then
    printf 'missing file: %s\n' "$path" >&2
    missing=1
  fi
done

if [[ "$missing" -ne 0 ]]; then
  exit 1
fi

printf 'repository layout verified\n'
