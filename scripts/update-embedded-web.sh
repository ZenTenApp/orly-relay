#!/usr/bin/env bash
# scripts/update-embedded-web.sh
# Build the embedded web UIs (relay dashboard + smesh client) and then install
# the Go binary.
#
# This script will:
#  - Build the Svelte relay dashboard in app/web to app/web/dist
#  - Build the Smesh client in app/smesh to app/smesh/dist
#  - Run `go install` from the repository root so the binary picks up the new
#    embedded assets.
#
# Usage:
#   ./scripts/update-embedded-web.sh
#
# Requirements:
#  - Go 1.18+ installed (for `go install` and go:embed support)
#  - Bun (https://bun.sh) recommended; alternatively Node.js with npm/yarn/pnpm
#
set -euo pipefail

# Resolve repo root to allow running from anywhere
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
WEB_DIR="${REPO_ROOT}/app/web"
SMESH_DIR="${REPO_ROOT}/app/smesh"

log() { printf "[update-embedded-web] %s\n" "$*"; }
err() { printf "[update-embedded-web][ERROR] %s\n" "$*" >&2; }

# Choose a JS package runner
JS_RUNNER=""
if command -v bun >/dev/null 2>&1; then
  JS_RUNNER="bun"
elif command -v npm >/dev/null 2>&1; then
  JS_RUNNER="npm"
elif command -v yarn >/dev/null 2>&1; then
  JS_RUNNER="yarn"
elif command -v pnpm >/dev/null 2>&1; then
  JS_RUNNER="pnpm"
else
  err "No JavaScript package manager found. Install Bun (recommended) or npm/yarn/pnpm."
  exit 1
fi

log "Using JavaScript runner: ${JS_RUNNER}"

# Helper: build a JS project in the given directory
build_js_project() {
  local dir="$1"
  local name="$2"

  if [[ ! -d "${dir}" ]]; then
    err "Expected ${name} directory at ${dir} not found."
    exit 1
  fi

  log "Building ${name}..."
  pushd "${dir}" >/dev/null
  case "${JS_RUNNER}" in
    bun)
      bun install
      bun run build
      ;;
    npm)
      npm ci || npm install
      npm run build
      ;;
    yarn)
      yarn install --frozen-lockfile || yarn install
      yarn build
      ;;
    pnpm)
      pnpm install --frozen-lockfile || pnpm install
      pnpm build
      ;;
    *)
      err "Unsupported JS runner: ${JS_RUNNER}"
      exit 1
      ;;
  esac
  popd >/dev/null

  local dist_dir="${dir}/dist"
  if [[ ! -d "${dist_dir}" ]]; then
    err "${name} build did not produce ${dist_dir}. Check the build configuration."
    exit 1
  fi
  log "${name} build complete at ${dist_dir}."
}

# Build relay dashboard (Svelte)
build_js_project "${WEB_DIR}" "relay dashboard"

# Build smesh client (React)
build_js_project "${SMESH_DIR}" "smesh client"

# Install the Go binary so it embeds the latest files
log "Running 'go install' from repo root..."
pushd "${REPO_ROOT}" >/dev/null
GO111MODULE=on go install ./...
popd >/dev/null

log "Done. Your installed binary now includes the updated embedded web UIs."
