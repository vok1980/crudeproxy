# crudeproxy

![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)

A small forward HTTP/HTTPS proxy that blocks requests by domain and logs
every connection. No MITM, no TLS interception, no external dependencies
beyond the Go standard library.

## What it does

- Filters HTTP requests by domain (blocks by exact match or any subdomain).
- Filters HTTPS via the `CONNECT` method by domain, without decrypting traffic.
- Logs every request to stdout or a file in a tab-separated format.
- Reloads the block list on `SIGHUP` without dropping active connections.
- Closes idle `CONNECT` tunnels after a configurable timeout, so a stuck
  tunnel does not leak file descriptors or goroutines.
- Optionally requires proxy authentication via the `Proxy-Authorization`
  header (HTTP Basic).

## What it does not do

- It does not decrypt HTTPS. Only the domain is visible for `CONNECT`
  requests, not the path, headers, or body.
- It does not authenticate clients. Run it on a trusted network or behind
  a firewall.
- It does not cache responses. Every request goes to the origin.

## Build

```bash
go build -o crudeproxy .
```

Requires Go 1.22 or newer. No external dependencies.

## Run

```bash
./crudeproxy -listen 127.0.0.1:8888 -block blocked.txt -log access.log
```

Default listen address is `127.0.0.1:8888`. The proxy refuses to start
silently on a public interface: if `-listen` is not a loopback address,
a warning is written to stderr.

Point clients at the proxy:

```bash
export http_proxy=http://127.0.0.1:8888
export https_proxy=http://127.0.0.1:8888
curl http://example.com
curl https://example.com
```

## Authentication

By default crudeproxy runs without authentication. To require proxy
credentials, pass `-auth-file` with a path to a users file:

```bash
./crudeproxy -auth-file /etc/crudeproxy/users.txt
```

File format:

```
# comments start with #
alice:secret
bob:hunter2
```

Passwords may contain `:`. Everything after the first colon is treated
as the password.

Clients authenticate with the standard `Proxy-Authorization` header:

```bash
curl --proxy-user alice:secret -x http://127.0.0.1:8888 http://example.com
export http_proxy=http://alice:secret@127.0.0.1:8888
```

The users file is reloaded on `SIGHUP` alongside the block list.

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `-listen` | `127.0.0.1:8888` | Address to listen on. |
| `-block` | `blocked.txt` | Path to the block list file. |
| `-auth-file` | *(empty)* | Proxy users file. Empty disables authentication. |
| `-log` | *(empty)* | Log file. Empty means stdout. |
| `-tunnel-idle-timeout` | `10m` | Idle timeout for `CONNECT` tunnels. `0` disables it. |

## Block list format

One domain per line. Lines starting with `#` are comments. Empty lines
are ignored. A leading or trailing dot is stripped. Domains are matched
case-insensitively.

```
# crudeproxy: blocked domains list

facebook.com
twitter.com
tiktok.com
doubleclick.net
```

A domain entry blocks the domain itself and all of its subdomains.
`facebook.com` matches `facebook.com`, `www.facebook.com`, and
`m.static.facebook.com`, but not `notfacebook.com`.

Reload the list without restarting:

```bash
kill -HUP $(pgrep crudeproxy)
```

## Log format

Tab-separated, one line per request:

```
2026/09/15 12:46:24  ALLOW  127.0.0.1  GET      example.com      200
2026/09/15 12:46:24  BLOCK  127.0.0.1  GET      facebook.com     http://facebook.com/
2026/09/15 12:46:24  ALLOW  127.0.0.1  CONNECT  example.com:443
2026/09/15 12:46:27  ERROR  127.0.0.1  CONNECT  unreachable.host  dial tcp: i/o timeout
```

Fields:

1. Timestamp
2. Status: `ALLOW`, `BLOCK`, or `ERROR`
3. Client IP
4. HTTP method (`GET`, `POST`, `CONNECT`, ...)
5. Target host (with port for `CONNECT`)
6. Extra info: HTTP status code for `ALLOW` on plain HTTP, full URL for
   `BLOCK`, error message for `ERROR`. Empty for `ALLOW` on `CONNECT`.

Top allowed domains:

```bash
awk '$3=="ALLOW" {print $6}' access.log | sed 's/:.*//' | sort | uniq -c | sort -rn | head
```

Blocked attempts:

```bash
awk '$3=="BLOCK" {print $6}' access.log | sort | uniq -c | sort -rn
```

Errors:

```bash
awk '$3=="ERROR" {print $6}' access.log | sort | uniq -c | sort -rn
```

## Tests

```bash
./test.sh
```

Runs `gofmt`, `go vet`, unit tests, then starts a real proxy and exercises
HTTP/HTTPS allow and block, `CONNECT` idle timeout, active transfer
survival, `SIGHUP` reload, and file descriptor leak detection.

Unit tests only:

```bash
go test ./...
```

The end-to-end script needs network access (`example.com`, `proof.ovh.net`)
and `curl`, `openssl`, and GNU `timeout`.

## Limitations

- No TLS interception. HTTPS filtering is domain-only.
- No caching.
- No IPv6-specific handling beyond what Go's `net` package provides.
- Passwords are stored in plaintext. Hash support is planned.

## License

MIT. See [LICENSE](LICENSE).

## Acknowledgments

Development of crudeproxy was assisted by Claude (Anthropic), used as a
coding assistant. All code has been reviewed, modified, and tested by
the author.
