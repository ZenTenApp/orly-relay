#!/bin/sh
# build.sh — Clean build of all tinyjs targets.
# Wipes compiled .mjs output before recompiling so no stale files survive.
# Hand-written $runtime/*.mjs files are preserved.
set -e
cd "$(dirname "$0")"

# Ensure tinyjs is available — build from submodule if missing.
if ! command -v tinyjs >/dev/null 2>&1; then
  echo "tinyjs not found, building from next/tinygo..."
  (cd next/tinygo && go build -o "$HOME/.local/bin/tinyjs" ./cmd/tinyjs)
fi

DIST=app/smesh3

# Generate version_gen.go in each tinyjs target — single source of truth from pkg/version/version.
VER=$(cat pkg/version/version)
for dir in next/sm3sh next/sw next/sw-relay; do
  printf 'package main\n\nconst version = "%s"\n' "$VER" > "$dir/version_gen.go"
done

# Wipe compiled .mjs files in each target dir (not subdirs — $runtime is safe).
# Preserve hand-written files: mls-bridge.mjs
for dir in "$DIST" "$DIST/\$sw" "$DIST/\$sw-relay"; do
  find "$dir" -maxdepth 1 -name '*.mjs' ! -name 'mls-bridge.mjs' -delete 2>/dev/null || true
done

# Recompile all targets.
echo "compiling sm3sh..."
(cd next/sm3sh && tinyjs -o ../../$DIST .)
echo "compiling sw (shell)..."
(cd next/sw && tinyjs -o ../../$DIST/\$sw .)
echo "compiling sw-relay..."
(cd next/sw-relay && tinyjs -o ../../$DIST/\$sw-relay .)

echo "build complete"
