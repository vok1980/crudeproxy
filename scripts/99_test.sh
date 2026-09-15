#!/usr/bin/env bash
set -euo pipefail

echo "==> binary"
ls -la /usr/local/bin/crudeproxy

echo "==> config and log dirs"
sudo ls -la /etc/crudeproxy /var/log/crudeproxy

echo "==> service status"
sudo systemctl status crudeproxy --no-pager || true

echo "==> listening socket"
if ! ss -tlnp | grep -q ':8888'; then
    echo "ERROR: nothing listening on port 8888" >&2
    exit 1
fi
ss -tlnp | grep ':8888'

echo "==> http allow"
code=$(curl -s -o /dev/null -w '%{http_code}' -x http://127.0.0.1:8888 http://example.com)
echo "http://example.com -> $code"
[ "$code" = "200" ] || { echo "ERROR: expected 200" >&2; exit 1; }

echo "==> http block"
code=$(curl -s -o /dev/null -w '%{http_code}' -x http://127.0.0.1:8888 http://ad.mail.ru)
echo "http://ad.mail.ru -> $code"
[ "$code" = "403" ] || { echo "ERROR: expected 403" >&2; exit 1; }

echo "==> reload"
sudo systemctl reload crudeproxy

echo "==> recent access log"
sudo tail -5 /var/log/crudeproxy/access.log

echo "==> recent journal"
sudo journalctl -u crudeproxy -n 20 --no-pager
