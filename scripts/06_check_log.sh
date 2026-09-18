#!/usr/bin/env bash
set -euo pipefail

echo "=============== top allowed =============="
sudo awk '$3=="ALLOW" {print $7}' /var/log/crudeproxy/access.log | sed 's/:.*//' | sort | uniq -c | sort -rn | head

echo "================= blocked ================"
sudo awk '$3=="BLOCK" {print $7}' /var/log/crudeproxy/access.log | sort | uniq -c | sort -rn

echo "================= failed ================="
sudo awk '$3=="ERROR" {print $7}' /var/log/crudeproxy/access.log | sort | uniq -c | sort -rn

echo "=============== authfail ================"
sudo awk '$3=="AUTHFAIL" {print $5, $4}' /var/log/crudeproxy/access.log | sort | uniq -c | sort -rn
