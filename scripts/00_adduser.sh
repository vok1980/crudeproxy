#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

adduser --system --group --no-create-home --shell /usr/sbin/nologin crudeproxy

mkdir -p /etc/crudeproxy
mkdir -p /var/log/crudeproxy

cp ${PROJECT_DIR}/blocked.txt /etc/crudeproxy/blocked.txt

chown -R crudeproxy:crudeproxy /etc/crudeproxy /var/log/crudeproxy
chmod 755 /etc/crudeproxy
chmod 750 /var/log/crudeproxy
