#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Create system user if it does not exist yet.
if ! getent passwd crudeproxy >/dev/null; then
    sudo adduser --system --group --no-create-home \
        --shell /usr/sbin/nologin crudeproxy
fi

sudo mkdir -p /etc/crudeproxy
sudo mkdir -p /var/log/crudeproxy

# Seed block list only on first install.
if [ ! -f /etc/crudeproxy/blocked.txt ]; then
    sudo cp "${PROJECT_DIR}/blocked.txt" /etc/crudeproxy/blocked.txt
fi

# Seed users file only on first install.
if [ ! -f /etc/crudeproxy/users.txt ]; then
    sudo tee /etc/crudeproxy/users.txt >/dev/null <<'EOF'
# crudeproxy: proxy users
# one user:password per line, # starts a comment
# example:
# alice:secret
EOF
fi

sudo chmod 600 /etc/crudeproxy/users.txt

sudo chown -R crudeproxy:crudeproxy /etc/crudeproxy /var/log/crudeproxy
sudo chmod 755 /etc/crudeproxy
sudo chmod 750 /var/log/crudeproxy
