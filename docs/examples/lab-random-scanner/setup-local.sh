#!/usr/bin/env bash
# Copy lab scanner reference files into gitignored local paths.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
SRC="$ROOT/docs/examples/lab-random-scanner"
mkdir -p "$ROOT/manifests/lab/random-scanner" "$ROOT/scripts/lab"
cp "$SRC/manifests/"*.yaml "$ROOT/manifests/lab/random-scanner/"
cp "$SRC/scripts/"*.sh "$ROOT/scripts/lab/"
chmod +x "$ROOT/scripts/lab/"*.sh
echo "Installed to manifests/lab/random-scanner/ and scripts/lab/ (gitignored)"
