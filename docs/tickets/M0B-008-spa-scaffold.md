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

- [ ] `npm run dev` in `web/` serves the shell; API calls reach a locally-running Go server through
      the proxy with no CORS configuration on either side.
- [ ] `npm run build` emits to `web/dist` with no TypeScript or ESLint errors.
- [ ] TypeScript `strict: true`, plus `noUncheckedIndexedAccess`. No `any` in committed code;
      `@ts-expect-error` and `eslint-disable` require an inline justification comment.
- [ ] Theme toggle works, survives reload, and both themes meet WCAG AA contrast for body text and
      primary buttons.
- [ ] The shell is usable at 1280px and degrades sensibly to 768px. Mobile is not a target.
- [ ] Keyboard navigation reaches every interactive element in the shell, with a visible focus ring.
- [ ] A thrown error in a child renders the error boundary, not a blank page.
- [ ] `npm test` runs and passes.

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
