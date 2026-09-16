#!/usr/bin/env bash
set -euo pipefail

sudo systemctl daemon-reload
sudo systemctl enable crudeproxy

if ! sudo systemctl start crudeproxy; then
    echo "crudeproxy failed to start; recent logs:" >&2
    sudo journalctl -u crudeproxy --no-pager -n 50 >&2 || true
    exit 1
fi

if ! sudo systemctl is-active --quiet crudeproxy; then
    echo "crudeproxy is not active after start" >&2
    sudo journalctl -u crudeproxy --no-pager -n 50 >&2 || true
    exit 1
fi
