# Integration Tests

These are the integration tests from the original [miniredis](https://github.com/alicebob/miniredis) Go implementation (`integration/`), adapted to compare miniredis-rs against miniredis-go at the RESP byte level.

## How it works

Each test starts two servers — miniredis-rs (as a subprocess) and miniredis-go (in-process) — then runs identical Redis commands against both and compares the raw RESP responses.

## Running

```bash
make test
```

This builds `miniredis-rs-server` (with TLS) and runs the Go tests with `INT=1` set. Tests are skipped without `INT=1`.

## Changes from upstream

The test files (`*_test.go`) and the comparison framework (`test.go`) are kept identical to upstream, with the following exceptions:

### `ephemeral.go`

Rewritten to start `miniredis-rs-server` as a subprocess (instead of `redis-server`). Config is passed via stdin; the server prints `PORT=<n>` to stdout when ready.

### `tls.go` and `testdata/`

The test certificates use a proper CA hierarchy because rustls's `WebPkiClientVerifier` rejects self-signed `CA:TRUE` certificates used as end-entity certs (which the upstream certs are). The upstream certs work with Go's `crypto/tls` but not with rustls.

- `ca.crt` / `ca.key` — self-signed CA
- `server.crt` / `server.key` — server cert signed by CA (`SAN: DNS:Server`)
- `client.crt` / `client.key` — client cert signed by CA

Both sides use `ca.crt` as the trust root.

### `generic_test.go`

`TestFastForward` uses `c.real.Do("MINIREDIS.FASTFORWARD", "200")` instead of `time.Sleep(200ms)`. miniredis-rs (like miniredis-go) uses mock time for TTLs, so wall-clock sleep doesn't expire keys. The custom `MINIREDIS.FASTFORWARD <ms>` command advances mock time on the subprocess, matching what `c.miniredis.FastForward()` does for miniredis-go in-process.
