#!/usr/bin/env bash
set -euo pipefail

systemctl daemon-reload
systemctl enable crudeproxy

if ! systemctl start crudeproxy; then
    echo "crudeproxy failed to start; recent logs:" >&2
    journalctl -u crudeproxy --no-pager -n 50 >&2 || true
    exit 1
fi

systemctl status crudeproxy --no-pager || true
