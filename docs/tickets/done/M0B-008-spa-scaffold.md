# M0B-008 — React + Vite + TypeScript + Tailwind + shadcn/ui scaffold

**Milestone:** M0b · **Size:** M · **Depends on:** M0B-001

## Why

`PLAN.md` replaces server-rendered pongo2 + jQuery with a real SPA. This ticket sets up the frontend
so that later feature work is writing components, not arguing about tooling. The application shell
built here — nav, layout, theme, error boundary, loading states — is reused by every screen in
M2–M6.

## Scope

**In**

- `web/` — Vite + React 18+ + TypeScript in **strict** mode.
- Tailwind CSS, shadcn/ui initialised (`components.json`), with a small set of primitives installed
  (button, input, dialog, table, badge, toast, dropdown-menu, tabs, form).
- Routing (`react-router`), with a layout route providing the app shell.
- App shell: top bar (product name, user menu placeholder), left nav (placeholder items), content
  outlet, toast host.
- Dark and light theme, following the OS preference with a manual override persisted to
  `localStorage`. Both must be usable — this tool gets used in dim rooms for hours.
- A global error boundary and a 404 route.
- Two demo screens proving the plumbing: a page showing `/version`, and one showing `/healthz`.
- ESLint + Prettier + `tsc --noEmit` wired into `npm run lint` and consumed by `make lint`.
- Vitest + React Testing Library, one meaningful test.
- Dev proxy: Vite proxies `/api` to the Go server so dev and prod are same-origin and no CORS is
  needed.

**Out**

- The generated API client (`M0B-009`) — the demo screens may use plain `fetch` and will be
  rewritten by that ticket. Note it in a comment.
- Auth screens (`M1-017`), any domain screen.

## Acceptance criteria

- [x] `npm run dev` in `web/` serves the shell; API calls reach a locally-running Go server through
      the proxy with no CORS configuration on either side.
- [x] `npm run build` emits to `web/dist` with no TypeScript or ESLint errors.
- [x] TypeScript `strict: true`, plus `noUncheckedIndexedAccess`. No `any` in committed code;
      `@ts-expect-error` and `eslint-disable` require an inline justification comment.
- [x] Theme toggle works, survives reload, and both themes meet WCAG AA contrast for body text and
      primary buttons.
- [x] The shell is usable at 1280px and degrades sensibly to 768px. Mobile is not a target.
- [x] Keyboard navigation reaches every interactive element in the shell, with a visible focus ring.
- [x] A thrown error in a child renders the error boundary, not a blank page.
- [x] `npm test` runs and passes.

## Tests

- One component test on the shell: renders nav, toggles theme, respects the persisted preference.
- One test asserting the error boundary catches and renders a fallback.

## Notes for the implementer

- shadcn/ui copies components into the repo rather than installing a dependency. Commit them and
  treat them as our code — edit freely, but keep edits minimal so future upstream diffs stay legible.
- Resist building a design system now. Install a primitive when a screen needs it.
- Keep `web/src/` organised by feature (`src/features/engagements/…`) rather than by type
  (`src/components/`, `src/hooks/`). The domain in `PLAN.md` §2 is big enough that type-based
  grouping stops scaling around M3.

---

## Implementation notes

### Toolchain

- **Node is pinned to 24.18.1** in [`.prototools`](../../../.prototools) and
  [`web/.nvmrc`](../../../web/.nvmrc), and stated again as an `engines` range in
  `web/package.json`. Current Vite requires `^20.19.0 || >=22.12.0`; the development machine was on
  22.3.0, which fails at install rather than at runtime. CI (`M0B-012`) and the image
  (`M0B-011`) should read the same pin rather than picking their own Node.
- **TypeScript is pinned to `~6.0`, not the current 7.x.** `typescript-eslint` supports
  `>=4.8.4 <6.1.0`; on TypeScript 7 the lint stack will not install at all. Lift the pin when
  `typescript-eslint` supports 7.
- **`eslint-plugin-jsx-a11y` was left out.** Its latest release peers on `eslint@^9` and this repo
  is on ESLint 10. The accessibility criteria are covered by tests instead (`app-shell.test.tsx`
  walks the tab order). Worth adding when the plugin catches up — that is the only reason it is
  absent.

### Deviations from scope

- **`form` was replaced by `field`.** The ticket lists `form` among the primitives to install. In
  the current shadcn registry `form` resolves to an empty stub — the React Hook Form wrapper was
  superseded by `field` (`Field`, `FieldLabel`, `FieldError`, …), which is form-library agnostic.
  `field` is installed instead, and pulled in `label` and `separator` as its dependencies. No form
  library is installed yet; `M1-017` is the first ticket with a form to build and should pick one.
- **`exactOptionalPropertyTypes` is not enabled.** The two the ticket names — `strict` and
  `noUncheckedIndexedAccess` — are on. `exactOptionalPropertyTypes` was tried and rejected: the
  vendored Radix components pass `T | undefined` into props typed `T`, so enabling it would mean
  editing every shadcn/ui component installed, which this ticket explicitly says not to do.
- **shadcn/ui style is `radix-nova`** (base `radix`, preset `nova`), which brings the Geist variable
  font. It is installed as `@fontsource-variable/geist` and bundled into `dist`, so nothing is
  fetched from a font CDN — required both by `font-src 'self'` and by the air-gapped deployment
  model.

### Content Security Policy and the theme

`internal/httpapi/headers.go` sends `script-src 'self'`, so the usual inline theme-bootstrap script
would be blocked in production and every dark-mode user would get a white flash on every load.
The bootstrap is therefore [`web/public/theme-bootstrap.js`](../../../web/public/theme-bootstrap.js),
an external file loaded synchronously ahead of the module bundle. It duplicates three constants from
`src/app/theme/theme.ts` (the storage key, the media query, the class name), and
`theme-bootstrap.test.ts` fails if they drift apart. No relaxation of the CSP was needed — the note
in `headers.go` inviting that argument can stay as it is.

### Repository-level changes this pulled in

- `web/node_modules` contains Go sources (the `flatted` package ships some), which `go test ./...`
  and `golangci-lint` both picked up — one npm dependency away from failing the Go build over code
  nobody here controls. The Makefile now lists `GO_PACKAGES` explicitly instead of `./...`, and
  `.golangci.yml` excludes the same path.
- `npm run generate` exists as a no-op that prints why, because `make generate` invokes it. M0B-009
  replaces it with the real client generation.

### Measured, not assumed

- **Contrast.** The palette is entirely achromatic, so Oklab `L` maps to linear-sRGB grey `L³` and
  the WCAG ratios are exact: body text 19.79:1 light / 18.96:1 dark, primary buttons 17.16:1 light /
  14.22:1 dark, secondary (`muted-foreground`) text 4.73:1 light / 7.63:1 dark. All clear AA (4.5:1),
  including the secondary text that comes closest.
- **The proxy.** Verified against a real `go run ./cmd/purpleops`: `GET /api/v1/version` and
  `GET /api/v1/healthz` through `http://localhost:5173` returned the server's own JSON and its
  security headers, with no CORS configuration on either side and no `Access-Control-*` header
  involved.

### Not verified in a browser

No browser tooling was available in the implementing session, so the shell was never *looked at*.
Layout at 1280px and 768px, the focus rings, and the absence of a theme flash follow from the code
and the component tests but have not been seen. `M0B-013` (Playwright) is the ticket that makes this
checkable in CI; until then a quick manual look at `npm run dev` is worth someone's two minutes.
