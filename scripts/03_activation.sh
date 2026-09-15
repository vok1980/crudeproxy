#!/usr/bin/env bash
set -euo pipefail

systemctl daemon-reload

systemctl enable crudeproxy

systemctl start crudeproxy

systemctl status crudeproxy
