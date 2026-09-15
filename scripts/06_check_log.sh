#!/usr/bin/env bash
set -euo pipefail

echo "=============== top allowed =============="
awk '$3=="ALLOW" {print $6}' /var/log/crudeproxy/access.log | sed 's/:.*//' | sort | uniq -c | sort -rn | head

echo "================= blocked ================"
awk '$3=="BLOCK" {print $6}' /var/log/crudeproxy/access.log | sort | uniq -c | sort -rn

echo "================= failed ================="
awk '$3=="ERROR" {print $6}' /var/log/crudeproxy/access.log | sort | uniq -c | sort -rn
