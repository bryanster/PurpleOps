# Security findings (2026-08-12)

Proven authorization failures in the current Blacklight tree. Each item below was
traced from OpenAPI → middleware → handler → store. Reproduction sketches are
concrete; they were not executed against a live instance in this pass.

**Out of scope / not listed:** style, missing tests, theoretical hardening, and
issues that require already-compromised host/operator access with no extra
privilege. Residual risks that are *not* proven vulns are at the bottom.

---

## Summary

| ID | Severity | Title |
|---|---|---|
| BL-001 | High | `GET /engagements` returns every engagement to any signed-in user |
| BL-002 | High | Nested workbook/report IDs are not bound to the authorized engagement |
| BL-003 | High | Production `Ownership.Facts` never resolves resource ownership or blind state |
| BL-004 | Medium | Content sync SSRF: admin-set source URL is fetched with no scheme/host allowlist |
| BL-005 | Medium | Share capability token is written to access logs; claim/password routes are public and unthrottled |

---

## BL-001 — `GET /engagements` enumerates every engagement

**Severity:** High  
**CWE:** CWE-639 (Authorization Bypass Through User-Controlled Key), CWE-863 (Incorrect Authorization)  
**Confidence:** High (code-path proof)

### Contract

`api/openapi.yaml` `listEngagements`:

```yaml
x-authz-self: true
x-authz-because: listing returns only what the caller can see; the handler filters by membership
```

The description says platform admins see every engagement, members see only
theirs, and non-members see an empty list — existence is supposed to stay
hidden (same concealment as 404 on `GET /engagements/{id}`).

`x-authz-self` makes `authorize()` allow any authenticated subject without
calling `authz.Can`:

```308:311:internal/httpapi/authorize.go
	if requirement.Self {
		authorization.Subject = subjectOf(caller, "", "", false)
		authorization.Allowed = true
		authorization.Reason = "the specification declares this operation self-service: " + requirement.Because
```

The exemption is only safe if the handler actually filters.

### Proof it does not filter

`ListEngagements` never reads `authn.SubjectFrom`. It forwards status/cursor/limit only:

```18:31:internal/httpapi/engagementhandlers.go
// ListEngagements returns a page of engagements the caller can see.
func (h *handlers) ListEngagements(...) (...) {
	var filter engagement.ListFilter
	// ... status / cursor / limit only ...
	items, err := h.engagements.List(ctx, filter)
```

The domain layer documents the missing split and then does not implement it:

```175:184:internal/engagement/service.go
// List returns a page of engagements visible to the caller.
// For admins this returns all engagements; for members only their own.
// The decision of which filter to apply is made by the handler ... not here.
func (s *Service) List(ctx context.Context, filter ListFilter) ([]storengagement.Engagement, error) {
	return s.engagements.List(ctx, storengagement.ListFilter{
		Status: filter.Status, After: filter.After, Limit: filter.Limit,
	})
}
```

The store query is globally unscoped:

```264:296:internal/store/engagement/engagements.go
const listEngagementsBase = selectEngagement + `WHERE 1=1 `
// ListFilter has Status, After, Limit — no user id, no membership join.
rows, err := r.db.Read().QueryContext(ctx, query, args...)
```

### Attacker

Any signed-in platform member, including a newly invited account with **zero**
engagement seats.

### Impact

The rest of the model answers non-members `404` so they cannot learn that an
engagement exists. This list leaks every engagement’s:

- id (UUIDv7 — input to BL-002)
- name, client, description
- dates, ATT&CK pin, `mode` (`blind` / `standard`)
- `createdBy`

That is the enumeration the 404 concealment is built to prevent. Blind-mode
existence is itself sensitive.

### Reproduction

1. Create `member-a` and `member-b` (platform role `member`).
2. As `member-a`, `POST /api/v1/engagements` → engagement `EA`.
3. Confirm `member-b` is not in `EA` (`GET /api/v1/engagements/{EA}` → 404).
4. As `member-b`: `GET /api/v1/engagements`.
5. **Expected:** empty `items`. **Actual:** `EA` is in the page, including
   client, description, mode, dates, `createdBy`.

### Fix

Filter in one place, not a comment:

- Pass the caller into `Service.List`.
- Admin (`PlatformRoleAdmin`): current query.
- Member: `JOIN app.engagement_member ON ... AND user_id = ?`.
- Do not leave this as “the handler will filter”.

---

## BL-002 — Cross-engagement IDOR on nested workbook and report objects

**Severity:** High  
**CWE:** CWE-639, CWE-284  
**Confidence:** High (code-path proof)

### How authorization is supposed to work

Nested routes are shaped `/engagements/{engagementId}/…/{childId}`. Middleware
authorizes `engagementId` (membership in *that* engagement). Handlers must then
bind `childId` to that same engagement. They do not.

Production `Ownership.Facts` (see BL-003) also never walks the child → parent
edge, so middleware cannot catch the mismatch either.

### Proof — scenarios

`GET`/`PATCH`/`DELETE /engagements/{engagementId}/scenarios/{scenarioId}`:

```76:80:internal/httpapi/scenariohandlers.go
func (h *handlers) GetScenario(...) {
	scenario, err := h.engagements.GetScenario(ctx, request.ScenarioId.String())
```

```67:70:internal/engagement/scenarios.go
func (s *Service) GetScenario(ctx context.Context, id string) (storengagement.Scenario, error) {
	return s.scenarios.ByID(ctx, id)
}
```

`PatchScenario` / `DeleteScenario` take only `scenarioId`. There is no
`scenario.EngagementID == path engagementId` check.

### Proof — write IDOR via CreateStep

```50:51:internal/httpapi/stephandlers.go
	in := engagement.CreateStepInput{
		ScenarioID: request.ScenarioId.String(),
```

`CreateStep` never verifies the scenario belongs to the authorized engagement.
A lead/red on engagement A who knows scenario UUID `SB` from engagement B
inserts a step into B.

Same shape on `GetStep` / `PatchStep` / `DeleteStep` (load by `stepId` only;
blind filter uses the *path* engagement’s mode, not the step’s real parent).

### Proof — reports

```75:79:internal/httpapi/reporthandlers.go
func (h *handlers) GetReport(...) {
	r, err := h.reports.Get(ctx, request.ReportId.String())
```

Preview is authorized on `engagementId`, then loads an arbitrary `reportId`:

```346:358:internal/httpapi/reporthandlers.go
func (h *handlers) previewReportEnv(ctx context.Context, engagementID, reportID string, ...) {
	eng, err := engagements.ByID(ctx, engagementID)
	// ...
	rep, err := h.reports.Get(ctx, reportID)
	// no rep.EngagementID == engagementID
```

The renderer then emits B’s draft blocks (rich text, findings, walkthrough
notes) to a member of A.

### Proof — template clone exfiltrates another engagement’s draft

`POST /engagements/{engagementId}/report-templates/from-report` is authorized
on the path engagement (caller’s A) and accepts any body `reportId`:

```180:184:internal/httpapi/templatehandlers.go
	tmpl, err := h.templates.CreateFromReport(ctx, report.CreateFromReportInput{
		EngagementID: request.EngagementId.String(),
		ReportID:     request.Body.ReportId.String(),
		Name:         request.Body.Name,
```

```255:264:internal/report/template_service.go
	reportBlocks, err := s.reports.BlocksByReport(ctx, in.ReportID)
	tmpl, err := s.templates.Create(ctx, storereport.NewTemplate{
		EngagementID: in.EngagementID, // caller’s A
		// blocks copied from in.ReportID with no engagement check
```

`ApplyTemplate` copies any `templateId`’s blocks onto the target report the
same way.

### Same class, other nested IDs

These OpenAPI mappings pass a *child* UUID as `x-authz-resource.engagement`.
Combined with BL-003, middleware looks up membership on a UUID that is not an
engagement:

| Route | `engagement:` value |
|---|---|
| `GET/DELETE /evidence/{evidenceId}` | `evidenceId` |
| `GET /evidence/{evidenceId}/content` | `evidenceId` |
| `GET/PATCH/DELETE /findings/{findingId}` | `findingId` |
| `GET /executions/{executionId}/evidence` | `executionId` |
| report/template/version/share object routes | `reportId` / `templateId` / `versionId` / `shareId` |

`GetEvidence` / `GetEvidenceContent` have **no** handler-side parent or
blind-reveal check. Authorization is entirely the broken Facts + Seat lookup.

### Attacker

A signed-in member (read) or lead/red (write) of engagement A who obtains a
child UUID from engagement B. UUIDv7 is not brute-forceable, but IDs leak via:

- BL-001’s global engagement list (then any subsequent UI/API that embeds child ids)
- activity feed, exports, archives, shared screens, logs
- report/template clone responses

### Impact

Cross-engagement confidentiality and integrity. A user confined to their own
client engagement can read another client’s scenarios/steps/report drafts and
clone narrative into their own engagement. A lead/red can inject or delete
workbook objects in an engagement they do not belong to.

### Reproduction

1. User A creates engagement `EA` with scenario `SA` and report `RA` containing
   unique rich text.
2. User B is lead only on engagement `EB`.
3. As B: `GET /api/v1/engagements/{EB}/scenarios/{SA}` → **SA’s body** (want 404).
4. As B: `POST /api/v1/engagements/{EB}/scenarios/{SA}/steps` → step appears
   under `EA`.
5. As B: `POST /api/v1/engagements/{EB}/reports/{RA}/preview` → HTML contains
   A’s draft.
6. As B: `POST /api/v1/engagements/{EB}/report-templates/from-report`
   `{"reportId": RA, "name": "stolen"}` → template in `EB` holds A’s blocks.

### Fix

Defense in depth, both layers:

1. Replace `membershipOwnership.Facts` (BL-003) so it resolves the named row to
   its owning engagement and compares that to the path engagement.
2. In every nested handler/repository, `404` when `row.EngagementID != path
   engagementId`.
3. For body-referenced `reportId` / `templateId`, require the same engagement
   as the path.

---

## BL-003 — Production `Ownership.Facts` is still the M1 stub

**Severity:** High (root control for BL-002 and inert blind-mode middleware)  
**CWE:** CWE-863, CWE-284  
**Confidence:** High

### Proof this is what production runs

`cmd/blacklight` never sets `Deps.Ownership`:

```116:121:cmd/blacklight/main.go
	handler, err := httpapi.NewServer(httpapi.Deps{
		Config:   cfg,
		Store:    db,
		Logger:   log,
		UI:       ui,
		Presence: presenceReg,
	})
```

`newServer` therefore installs the membership-table stub:

```141:144:internal/httpapi/server.go
	if deps.Ownership == nil {
		// Memberships are enough until M3 owns engagements themselves.
		deps.Ownership = NewMembershipOwnership(identity.NewMemberships(deps.Store))
	}
```

That stub’s `Facts` accepts any path string as a real engagement and hard-codes
not-blind / fully revealed:

```19:53:internal/httpapi/ownership.go
// Engagements themselves do not exist as rows until M3, so [Facts] accepts any
// engagement id the path named and reports it as not blind and fully revealed.
func (membershipOwnership) Facts(_ context.Context, ref ResourceRef) (ResourceFacts, error) {
	if ref.EngagementID == "" {
		return ResourceFacts{}, apierr.NotFound("engagement", ref.ID)
	}
	return ResourceFacts{
		EngagementID: ref.EngagementID,
		Blind:        false,
		Revealed:     true,
	}, nil
}
```

M3 engagements *do* exist as rows. This comment is stale; the loader is not.

### Why it matters

`facts()` copies `Facts.EngagementID` / `Blind` / `Revealed` into `authz.Resource`.
`authz.Can` then:

- looks up `subject.MembershipIn(resource.EngagementID)` — if OpenAPI labelled
  `evidenceId` as `engagement`, Seat is queried for a UUID that is not an
  engagement → non-member → 404 for everyone who is not a platform admin, **or**
  a false “member” if that UUID ever collides with a real engagement id;
- applies `GuardBlindMode` only when `EngagementBlind && !Revealed`. Production
  Facts always sets the opposite, so **middleware never 404s a blue caller**.

Some handlers re-implement `stepBlindScope` (steps, executions, comments,
analytics). `GetEvidence` / `GetEvidenceContent` do not — they rely on this
middleware.

### Reproduction (control failure)

Compare `authorize_test` `stubOwnership` (sets `Blind` from the engagement row)
with production `membershipOwnership` (always `Blind=false`, `Revealed=true`).

Call `GET /api/v1/evidence/{evidenceId}` as blue in a blind engagement:

1. Middleware never applies `GuardBlindMode` (`Facts.Blind` is false).
2. Seat is looked up on the *evidence* UUID, not the owning engagement.
3. Handler streams the blob with no reveal check.

### Fix

Implement `Ownership.Facts` by loading the named resource and returning its
real engagement id, mode, and reveal flag. Point
`x-authz-resource.engagement` at the actual engagement path param (or a
dedicated owner field the loader understands). Keep handler-side parent
equality checks.

---

## BL-004 — Content sync SSRF (admin-gated)

**Severity:** Medium  
**CWE:** CWE-918  
**Confidence:** High  
**Prerequisite:** `content.manage` (platform admin, or a token with
`content:write` whose owner is admin).

### Proof

`PATCH /content/sources/{sourceId}` accepts an arbitrary `url` (`maxLength:
2000`, no scheme/host constraint). `UpdateSource` stores it unchanged:

```129:131:internal/content/registry.go
	if edit.URL != nil && *edit.URL != before.URL {
		delta["url"] = change(before.URL, *edit.URL)
		after.URL = *edit.URL
```

`POST .../sync` then GETs that URL via `HTTPSource` with `http.DefaultClient`
when the runner has no injected client:

```909:913:internal/content/runner.go
func (r *Runner) httpClient() HTTPDoer {
	if r.http != nil {
		return r.http
	}
	return http.DefaultClient
}
```

```38:56:internal/content/bytesource.go
func (s HTTPSource) Open(ctx context.Context) (io.ReadCloser, int64, error) {
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	res, err := client.Do(req)
```

No private-IP deny list, no scheme allowlist (`https` only), no redirect
policy. `http.DefaultClient` follows redirects. ATT&CK adapter concatenates
`base + "/" + ref` (`internal/content/attack/adapter.go` `buildBundleURL`), so
`url=http://169.254.169.254` / `http://127.0.0.1:8080/...` / `file:///etc/passwd`
are all attempted. Response bytes are parsed as content, but job failure
messages and stored raw snapshots still reflect what was fetched.

### Impact

An admin session or a leaked `content:write` token belonging to an admin can
use the server as an HTTP client against link-local metadata, loopback
admin ports, or `file:` URLs. In cloud deployments that is credential theft
(IMDS). On a host with an unauthenticated loopback service it is RCE-adjacent.

Admin is a privileged role, but this is still a confused deputy: the *server*
makes the request, not the attacker’s browser.

### Reproduction

1. Sign in as platform admin.
2. `PATCH /api/v1/content/sources/{attackSourceId}`
   `{"url":"http://127.0.0.1:9"}` (or IMDS / `file:`).
3. `POST /api/v1/content/sources/{id}/sync`.
4. Observe outbound GET from the Blacklight process; job error / raw snapshot
   carries response body (or connection refused / file contents).

### Fix

- Allow only `https` (and `http` in development).
- Reject loopback, link-local, RFC1918, and metadata hostnames.
- Custom `CheckRedirect` that re-applies the same checks.
- Do not use `http.DefaultClient` in production.

---

## BL-005 — Share tokens land in access logs; claim/password are public and unthrottled

**Severity:** Medium  
**CWE:** CWE-532 (Insertion of Sensitive Information into Log File), CWE-307  
**Confidence:** High

### Proof — token in logs

Share claim URLs are `/api/v1/report-views/{token}`. The token is 256 bits of
hex (`ShareTokenHasher.Generate`). The access logger records `r.URL.Path`,
explicitly to *avoid* query-string secrets — path tokens were not considered:

```34:39:internal/httpapi/logging.go
				log.LogAttrs(ctx, slog.LevelInfo, "request",
					slog.String("method", r.Method),
					// Path, not RequestURI: a query string can carry a token, a
					// share secret or a filter nobody meant to publish ...
					slog.String("path", r.URL.Path),
```

`authorize.go` refusal logs and `apierr` problem logs also include `r.URL.Path`.
Anyone who can read process logs (central aggregator, support dump, debug
level left on) gets live share tokens.

### Proof — public, unthrottled password oracle

These operations are `x-authz-public` with `security: []`:

- `GET /report-views/{token}` — existence + `passwordRequired`
- `POST /report-views/{token}/claim` — CSRF-exempt
- `POST /report-views/{token}/password` — CSRF-exempt; Argon2id verify

`credentialAccounts` throttles only login + MFA verify. Share password verify
is unlimited. `CreateReportShare.password` is documented “at least 8
characters” but the schema has **no** `minLength`, and `CreateShare` hashes
without `password.Validate`.

`GetReportShareHtml` / `GetReportSharePdf` do require a grant, so a leaked
token alone does not dump HTML. Combined with any account (or the unimplemented
guest-register once it ships), `ClaimShare` binds a grant. Password, if set,
is the only remaining gate — and it can be guessed without rate limit.

Grant cap (`maxGrants`) is check-then-insert with no unique/transactional
constraint (`app.report_share_grant` has only indexes). Concurrent claims can
exceed `maxGrants`.

### Impact

Log access → share token. Token + any local account → claim (or password
guess against Argon2id with no lockout) → published report HTML/PDF. Weak
share passwords are practical; the 256-bit token is not, until it hits a log.

### Reproduction

1. Create a password-gated share; note the claim URL token.
2. `GET /api/v1/report-views/{token}` with no cookie → `exists: true`,
   `passwordRequired: true`.
3. Inspect the server info log line: `path` contains the full token.
4. `POST /api/v1/report-views/{token}/password` in a loop with wrong
   passwords → every guess is processed (Argon2id, no 429).
5. After a correct guess (or claim with password), HTML/PDF are granted.

### Fix

- Log a redacted path (`/report-views/[redacted]`) for share routes.
- Throttle `claim` and `password` by token prefix + source IP, same limiter
  as login.
- Enforce `password.Validate` (or at least `minLength: 8`) on share passwords.
- Enforce `maxGrants` inside the insert transaction (`SELECT … FOR UPDATE` /
  `COUNT` + insert in one write, or a trigger).

---

## What was reviewed and is *not* a proven vuln

These were examined; do not treat them as open High/Critical items.

| Area | Result |
|---|---|
| Session cookies / CSRF HMAC double-submit | Sound. Route-explicit exemptions. Mutating-route walk test exists. |
| OIDC/SAML | Redirect URI from `BLACKLIGHT_BASE_URL`; PKCE/nonce; SAML sig/audience/recipient/`InResponseTo`; `returnto.Safe` refuses `//`, `\`, schemes. |
| Service tokens | HMAC at rest; two-fence authz; session-only guards on token/session management. |
| SPA path traversal | Map lookup after `path.Clean`; tests for `../` and `%2e%2e`. |
| Evidence blob paths | Content-addressed SHA-256; no user path in `blobPath`. |
| Archive ZIP | Writer only (export); no extract-to-disk of engagement archives. |
| HTML sanitizer | Bluemonday allowlist; write + render. Finding severity interpolated into a class is enum-constrained by OpenAPI. Branding colours validated `#RRGGBB`. |
| SQL | Parameterized; dynamic `SET` lists are hardcoded column names. |
| Compose first-boot secrets | Generated from `/dev/urandom` onto the data volume; documented as laptop-only. |
| Guest registration | Public in OpenAPI; handler returns “not yet implemented”. Not a live signup hole. |
| Share password cookie | Set by verify, **never checked** on HTML/PDF (`canAccessSharedVersion` is grant-only). Password is enforced at *claim*. Residual: a grant holder does not re-prove the password. Documented as deferred in M6-012. |

---

## Suggested fix order

1. **BL-001** — one query change; stops the engagement-id leak that feeds BL-002.
2. **BL-003** — real `Ownership.Facts`; unblocks correct middleware decisions.
3. **BL-002** — parent-bind every nested ByID (and body-referenced report/template ids).
4. **BL-005** — redact share tokens in logs; throttle claim/password.
5. **BL-004** — egress allowlist on content HTTP.

Add regression tests that fail on the reproduction steps above. The existing
authz matrix does not cover list filtering or child-id/parent-id equality.
