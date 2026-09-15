#!/usr/bin/env bash
#
# test.sh - end-to-end tests for crudeproxy.
#
# Requires: go, curl, openssl, timeout (GNU coreutils), date with %N support.
# Uses real network for HTTP/HTTPS tests (example.com, proof.ovh.net).

set -u

PROXY_PORT="${PROXY_PORT:-18888}"
PROXY_ADDR="127.0.0.1:${PROXY_PORT}"
PROXY_URL="http://${PROXY_ADDR}"
IDLE_TIMEOUT=3s
LOG_FILE=$(mktemp -t crudeproxy-log.XXXXXX)
BLOCK_FILE=$(mktemp -t crudeproxy-block.XXXXXX)
PROXY_PID=""
FAILED=0

cleanup() {
    if [ -n "${PROXY_PID:-}" ] && kill -0 "$PROXY_PID" 2>/dev/null; then
        kill "$PROXY_PID" 2>/dev/null || true
        wait "$PROXY_PID" 2>/dev/null || true
    fi
    rm -f "$LOG_FILE" "$BLOCK_FILE" "${AUTH_FILE:-}"
}
trap cleanup EXIT

pass() { printf '  \033[32mPASS\033[0m %s\n' "$1"; }
fail() { printf '  \033[31mFAIL\033[0m %s\n' "$1"; FAILED=1; }
info() { printf '==> %s\n' "$1"; }

require() {
    command -v "$1" >/dev/null 2>&1 || { echo "missing required tool: $1" >&2; exit 1; }
}

require go
require curl
require openssl
require timeout
require date

info "build"
if ! go build -o crudeproxy . ; then
    echo "build failed" >&2
    exit 1
fi

info "gofmt check"
if [ -n "$(gofmt -l .)" ]; then
    fail "gofmt reports unformatted files: $(gofmt -l .)"
else
    pass "gofmt clean"
fi

info "go vet"
if go vet ./... ; then
    pass "go vet clean"
else
    fail "go vet found issues"
fi

info "unit tests"
if go test ./... ; then
    pass "unit tests passed"
else
    fail "unit tests failed"
    exit 1
fi

info "start proxy"
cat > "$BLOCK_FILE" <<'EOF'
facebook.com
twitter.com
EOF

./crudeproxy -listen "$PROXY_ADDR" -block "$BLOCK_FILE" -log "$LOG_FILE" \
    -tunnel-idle-timeout "$IDLE_TIMEOUT" &
PROXY_PID=$!

# Wait until the proxy is reachable.
ready=0
for _ in $(seq 1 50); do
    if kill -0 "$PROXY_PID" 2>/dev/null && \
       curl -s -o /dev/null --max-time 1 -x "$PROXY_URL" http://example.com/ 2>/dev/null; then
        ready=1
        break
    fi
    sleep 0.1
done

if [ "$ready" -ne 1 ]; then
    echo "proxy failed to start" >&2
    cat "$LOG_FILE" >&2 || true
    exit 1
fi
pass "proxy is listening on $PROXY_ADDR"

# ---------------------------------------------------------------------------
info "HTTP allow"
code=$(curl -s -o /dev/null -w '%{http_code}' -x "$PROXY_URL" --max-time 10 http://example.com/)
if [ "$code" = "200" ]; then pass "http://example.com -> 200"; else fail "http://example.com -> $code"; fi

info "HTTP block"
code=$(curl -s -o /dev/null -w '%{http_code}' -x "$PROXY_URL" --max-time 10 http://facebook.com/)
if [ "$code" = "403" ]; then pass "http://facebook.com -> 403"; else fail "http://facebook.com -> $code"; fi

info "HTTPS allow"
code=$(curl -s -o /dev/null -w '%{http_connect}' -x "$PROXY_URL" --max-time 10 https://example.com/)
if [ "$code" = "200" ]; then pass "https://example.com -> 200"; else fail "https://example.com -> $code"; fi

info "HTTPS block"
code=$(curl -s -o /dev/null -w '%{http_connect}' -x "$PROXY_URL" --max-time 10 https://twitter.com/)
if [ "$code" = "403" ]; then pass "https://twitter.com -> 403"; else fail "https://twitter.com -> $code"; fi

# ---------------------------------------------------------------------------
info "idle timeout ($IDLE_TIMEOUT)"
start_ms=$(date +%s%3N)
timeout 15 openssl s_client -proxy "$PROXY_ADDR" -connect example.com:443 -quiet </dev/null >/dev/null 2>&1 || true
end_ms=$(date +%s%3N)
elapsed_ms=$(( end_ms - start_ms ))
# Correct: ~ IDLE_TIMEOUT + handshake. Buggy (2*idle): ~6000ms.
if [ "$elapsed_ms" -ge 1500 ] && [ "$elapsed_ms" -lt 5000 ]; then
    pass "tunnel closed after ${elapsed_ms}ms"
else
    fail "idle timeout elapsed ${elapsed_ms}ms (want 1500-5000)"
fi

# ---------------------------------------------------------------------------
info "active transfer survives idle timeout"
# 1MB at ~100 kB/s takes ~10s, well beyond the 3s idle window.
if curl -s -o /dev/null -x "$PROXY_URL" --max-time 60 --limit-rate 100k \
        https://proof.ovh.net/files/1Mb.dat; then
    pass "rate-limited 1MB download completed"
else
    fail "rate-limited download failed (idle timeout killed active transfer?)"
fi

# ---------------------------------------------------------------------------
info "SIGHUP reload"
echo "example.org" >> "$BLOCK_FILE"
kill -HUP "$PROXY_PID"
# Give the reload a moment.
sleep 0.3
code=$(curl -s -o /dev/null -w '%{http_code}' -x "$PROXY_URL" --max-time 10 http://example.org/)
if [ "$code" = "403" ]; then
    pass "example.org blocked after SIGHUP"
else
    fail "example.org -> $code after SIGHUP"
fi

# ---------------------------------------------------------------------------
info "FD leak check"
if [ -d "/proc/$PROXY_PID/fd" ]; then
    fds_before=$(ls "/proc/$PROXY_PID/fd" | wc -l)
    timeout 10 openssl s_client -proxy "$PROXY_ADDR" -connect example.com:443 -quiet </dev/null >/dev/null 2>&1 || true
    sleep 0.5
    fds_after=$(ls "/proc/$PROXY_PID/fd" | wc -l)
    if [ "$fds_before" = "$fds_after" ]; then
        pass "FD count unchanged ($fds_before -> $fds_after)"
    else
        fail "FD count changed ($fds_before -> $fds_after)"
    fi
else
    pass "skipped (no /proc)"
fi

# ---------------------------------------------------------------------------
info "authentication"
AUTH_FILE=$(mktemp -t crudeproxy-users.XXXXXX)
echo "alice:secret" > "$AUTH_FILE"
chmod 600 "$AUTH_FILE"

# Restart proxy with auth
kill "$PROXY_PID" 2>/dev/null || true
wait "$PROXY_PID" 2>/dev/null || true
./crudeproxy -listen "$PROXY_ADDR" -block "$BLOCK_FILE" -log "$LOG_FILE" \
    -auth-file "$AUTH_FILE" -tunnel-idle-timeout "$IDLE_TIMEOUT" &
PROXY_PID=$!
sleep 0.3

code=$(curl -s -o /dev/null -w '%{http_code}' -x "$PROXY_URL" --max-time 5 http://example.com/ || true)
if [ "$code" = "407" ]; then pass "no credentials -> 407"; else fail "no credentials -> $code"; fi

code=$(curl -s -o /dev/null -w '%{http_code}' -x "$PROXY_URL" --proxy-user alice:wrong --max-time 5 http://example.com/ || true)
if [ "$code" = "407" ]; then pass "wrong password -> 407"; else fail "wrong password -> $code"; fi

code=$(curl -s -o /dev/null -w '%{http_code}' -x "$PROXY_URL" --proxy-user alice:secret --max-time 10 http://example.com/)
if [ "$code" = "200" ]; then pass "valid credentials -> 200"; else fail "valid credentials -> $code"; fi

code=$(curl -s -o /dev/null -w '%{http_connect}' -x "$PROXY_URL" --proxy-user alice:secret --max-time 10 https://example.com/)
if [ "$code" = "200" ]; then pass "CONNECT with credentials -> 200"; else fail "CONNECT with credentials -> $code"; fi

rm -f "$AUTH_FILE"

# ---------------------------------------------------------------------------
echo
if [ "$FAILED" -eq 0 ]; then
    printf '\033[32mall tests passed\033[0m\n'
    exit 0
else
    printf '\033[31msome tests failed\033[0m\n'
    echo "--- proxy log ---"
    cat "$LOG_FILE" || true
    exit 1
fi
