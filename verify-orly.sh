 #!/usr/bin/env bash
   set -euo pipefail

   BIN="${1:-./orly}"
   echo "binary: $(readlink -f "$BIN")"

   echo
   echo "== build settings (want trimpath pointing at the repo root) =="
   go version -m "$BIN" | grep -E 'trimpath|gcflags|asmflags' || true

   echo
   echo "== embedded repo source paths (want pkg/..., cmd/..., app/...) =="
   strings "$BIN" | grep -E '^(pkg|cmd|app|lib)/[^ ]*\.go$' | sort -u | head -10

   echo
   echo "== embedded module-path source paths =="
   strings "$BIN" | grep -E '^git\.smesh\.lol/orly/[^ ]*\.go$' | head -5 || true

   echo
   echo "== runtime log probe =="
   # invalid ORLY_PORT makes config parsing fail before DB/network startup.
   # LOG_LEVEL=trace is needed because config.New uses chk.T (trace) for this error.
   LOG_LEVEL=trace ORLY_PORT=notanumber timeout 10 env -u ORLY_LOG_FILE "$BIN" relay 2>&1 | head -5
