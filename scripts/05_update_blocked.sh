#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cp ${PROJECT_DIR}/blocked.txt /etc/crudeproxy/blocked.txt
systemctl reload crudeproxy
journalctl -u crudeproxy -n 5 --no-pager
