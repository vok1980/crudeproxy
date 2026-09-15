#!/usr/bin/env bash
set -euo pipefail

sudo tee /etc/logrotate.d/crudeproxy >/dev/null <<'EOF'
/var/log/crudeproxy/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0640 crudeproxy crudeproxy
    copytruncate
}
EOF
