#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

sudo cp "${PROJECT_DIR}/blocked.txt" /etc/crudeproxy/blocked.txt
sudo systemctl reload crudeproxy
sudo journalctl -u crudeproxy -n 5 --no-pager
