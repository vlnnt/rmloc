#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"

echo "Building RMLOC for Linux..."

OUTPUT_DIR="rmloc-linux"

mkdir -p "$OUTPUT_DIR"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o "$OUTPUT_DIR/rmloc" \
    ./cmd/rmloc

chmod +x "$OUTPUT_DIR/rmloc"

if [ -d "prompts" ]; then
    cp -r prompts "$OUTPUT_DIR/"
fi

echo
echo "Build completed:"
echo "  $OUTPUT_DIR/rmloc"
echo "  $OUTPUT_DIR/prompts/"
