# internal/adapter/github

Implements `mtt.ReleaseSource` over the GitHub Releases HTTP API
(`repos/pashukhin/mtt/releases/latest`). Maps the API JSON (`tag_name`,
`assets[].{name, browser_download_url}`) into `mtt.Release`.

## Boundaries
- The HTTP client is an injectable `httpDoer` — tests use a fake (no socket; the
  "no network in tests" rule). `New()` uses the default `*http.Client`.
- Per-operation context deadlines: `apiTimeout` (metadata) vs `downloadTimeout`
  (assets). No global `Client.Timeout`; `CheckRedirect`/`Transport.Proxy` left at
  defaults so `browser_download_url` redirects and `HTTP(S)_PROXY` work.
- No auth in v1 (unauthenticated 60 req/hr/IP); `GH_TOKEN` support is deferred.
