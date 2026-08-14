# M7-014 — Content sync SSRF allowlist

**Milestone:** M7 · **Size:** M · **Depends on:** M2-002, M2-003  
**Finding:** [BL-004](../SECURITY_FINDINGS.md#bl-004--content-sync-ssrf-admin-gated) · **Severity:** Medium

## Why

`PATCH /content/sources/{id}` stores an arbitrary `url` (OpenAPI `maxLength: 2000`, no scheme or
host constraint). Sync then `GET`s it with `http.DefaultClient`: follows redirects, no private-IP
deny list, no scheme allowlist. ATT&CK concatenates `base + "/" + ref`.

The caller must hold `content.manage` (platform admin, or a `content:write` token whose owner is
admin). That is still a confused deputy: the **server** fetches IMDS, loopback, or `file:` URLs.

## Scope

**In**

- Validate source URLs on write (`UpdateSource`) and again at fetch time (defense in depth —
  a row written before this ticket must not stay dangerous).
- Allow `https` always. Allow `http` only when `BLACKLIGHT_ENV=development`.
- Reject loopback, link-local (`169.254.0.0/16`, `fe80::/10`), RFC1918, unique-local IPv6,
  metadata hostnames (`169.254.169.254`, `metadata.google.internal`, …).
- Custom `CheckRedirect` that re-applies the same checks on every hop.
- Do not use `http.DefaultClient` for content fetch in production. Give the runner a client
  with timeout + redirect policy.
- Refuse `file:`, `gopher:`, and any non-http(s) scheme.
- Job failure messages must not echo response bodies from rejected or private targets.

**Out**

- Changing who may call `content.manage`.
- Offline bundle upload / reprocess (already local-path fenced).
- OIDC/SAML egress (operator-configured IdP URLs; different trust model).

## Files

- `internal/content/registry.go` — validate on `UpdateSource`
- `internal/content/bytesource.go` — validate in `HTTPSource.Open`
- `internal/content/runner.go` — production HTTP client, not `DefaultClient`
- `internal/content/attack/adapter.go` — `buildBundleURL` result must pass the same check
- `internal/config` — only if an operator escape hatch is required (default: no hatch)
- Tests in `internal/content`

## Acceptance criteria

- [x] `PATCH` with `url=http://127.0.0.1/` is 400 in production config.
- [x] `PATCH` with `url=http://169.254.169.254/latest/meta-data/` is 400.
- [x] `PATCH` with `url=file:///etc/passwd` is 400.
- [x] `PATCH` with `url=https://github.com/...` still succeeds.
- [x] A pre-existing row pointed at a private URL fails closed at sync start (no packet to the
      target if DNS/IP is private; no `file:` open).
- [x] A 302 from an allowed host to `http://169.254.169.254/` is not followed.
- [x] Production runner does not call `http.DefaultClient`.

## Tests

Table-driven URL tests: public https (ok), http in prod (no), http in development (ok),
loopback, RFC1918, link-local, IPv6 ULA, `file:`, redirect-to-private.

Do not hit the network. Inject `HTTPDoer`.

## Notes for the implementer

DNS rebinding: resolve and pin, or reject if any resolved address is private. A hostname that
resolves to both public and private is a deny.

Do not special-case “admin said so” with a config flag unless an operator ticket asks for it.
The whole point is that admin UI is not an HTTP proxy.

Medium: not a silent defer, but not a `M7-009` blocker unless the ship owner promotes it.

## Implementation notes

- New `internal/content/urlpolicy.go` owns the fence: `URLPolicy.Validate` (scheme allowlist,
  metadata-host blocklist, private/reserved-IP deny, DNS-rebinding deny when any resolved address
  is private) and `URLPolicy.NewClient` (redirect re-validation + dial-time pinning + dial and
  response-header timeouts). Wired on write (`Registry.UpdateSource`) and on fetch
  (`HTTPSource.Open`, plus every adapter passing `FetchRequest.Policy` through).
- `http` is allowed only when `config.Environment.IsDevelopment()`; the policy is derived from
  `deps.Config.Env` in `internal/httpapi/server.go`. `blctl` builds its runner without a policy, so
  it keeps the strictest default (https only) — it does not read `BLACKLIGHT_ENV`.
- No whole-request `http.Client.Timeout`: the response body is bounded by the job context (and
  `MaxBytes`), and a fixed client timeout would truncate legitimate multi-hundred-MiB catalogs on
  slow links. The client still carries dial (10s) and response-header (30s) timeouts, which is what
  stops a private host that accepts TCP but never answers.
- Tests never touch the network: `URLPolicy` takes an injectable `LookupIP`, and the httpapi test
  server stubs `Deps.ContentLookupIP` by default (override it for fence-specific cases).
