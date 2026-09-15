#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd ${PROJECT_DIR}
go build -ldflags="-s -w" -o crudeproxy .
install -m 755 crudeproxy /usr/local/bin/crudeproxy
