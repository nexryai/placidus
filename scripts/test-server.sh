#!/usr/bin/env bash
set -euo pipefail
test_data_dir=$(mktemp -d)
trap 'rm -rf "$test_data_dir"' EXIT
export PLACIDUS_DATA_DIR="$test_data_dir"
export PLACIDUS_LISTEN=127.0.0.1:40774
export PLACIDUS_MIRAKURUN_URL=http://127.0.0.1:1
export PLACIDUS_FFMPEG=/nonexistent/placidus-test-ffmpeg
go run ./cmd/placidus
