# web/

The single-page app: React, Vite, TypeScript, Tailwind and shadcn/ui. `web/dist` is embedded into
the server binary via `embed.FS` (M0B-010), so a release is one file.

## Running it

Node is pinned in [`../.prototools`](../.prototools) and [`.nvmrc`](.nvmrc), and stated again as an
`engines` range in `package.json` — an older Node fails at `npm install` rather than halfway through
a build.

```sh
npm ci
npm run dev     # http://localhost:5173
```

`npm run dev` proxies `/api` to a Go server on `http://localhost:8080`, so dev and production are
both same-origin and neither side has any CORS configuration. Point it elsewhere with
`PURPLEOPS_DEV_PROXY_TARGET`. Start the server with:

```sh
go run ./cmd/purpleops     # needs PURPLEOPS_BASE_URL and friends — see ../.env.example
```

| Command              | What it does                                             |
| -------------------- | -------------------------------------------------------- |
| `npm run dev`        | Vite dev server with the `/api` proxy                    |
| `npm run build`      | Type-check, then build to `dist/`                        |
| `npm run lint`       | `tsc --noEmit`, ESLint, and a Prettier check             |
| `npm run format`     | Rewrite files with Prettier                              |
| `npm run generate`   | Regenerate `src/api/schema.d.ts` from `api/openapi.yaml` |
| `npm test`           | Vitest, once                                             |
| `npm run test:watch` | Vitest, watching                                         |

`make lint test build` at the repo root runs the Go and web halves together, which is what CI does.

## Layout

Organised by feature, not by file type — the domain in `PLAN.md` §2 is large enough that a single
`components/` directory stops scaling around M3.

```
public/theme-bootstrap.js   Sets the theme before first paint (see "Theming")
src/api/                    The generated client — see src/api/README.md
src/app/                    The application shell: nav, layout, theme, error boundary, routes
src/components/ui/          Vendored shadcn/ui primitives — see below
src/features/<feature>/     One directory per feature. Screens, and the data they need
src/lib/                    Small shared helpers with no feature knowledge
src/test/                   Vitest setup, the MSW fixture library, shared test utilities
```

## Talking to the API

Read [`src/api/README.md`](src/api/README.md) before writing a screen that loads anything. The short
version:

- `src/api/schema.d.ts` is generated from `../api/openapi.yaml` by `npm run generate`, which
  `make generate` runs and CI compares against a fresh run. Never edit it.
- **No component calls the API.** A feature exports `queryOptions()` factories and hooks from its
  `queries.ts`, and components import the hook.
- **No file writes an API URL.** `src/api/client.ts` is the only one that knows what one looks like,
  and ESLint rejects an `/api/…` literal or a bare `fetch` anywhere else.
- Errors arrive as `ApiError` with the problem document's `code`; a 401 and a 5xx are handled once,
  globally, in `src/api/query-provider.tsx`.

Tests run against MSW handlers built from the same generated types (`src/test/msw/`), so a fixture
cannot describe a response the real server would not send. An unhandled request fails the test.

## shadcn/ui

`npx shadcn@latest add <name>` copies a component into `src/components/ui/` and it is then ours to
commit and edit. Keep edits minimal so a future upstream diff stays legible — `sonner.tsx` carries
one, marked with a comment, because upstream reads `next-themes` and we have our own theme context.

Install a primitive when a screen needs one. Do not add them speculatively.

Prettier and some of the stricter lint rules are switched off for that directory (see
`.prettierignore` and `eslint.config.js`); everything there is still type-checked.

## Theming

Light and dark both matter — this tool gets used in dim rooms for hours. The preference is `light`,
`dark`, or `system`, persisted to `localStorage` under `purpleops.theme`, and it resolves to a
`dark` class on `<html>`.

The awkward part is the first paint. The server sends `script-src 'self'`
(`internal/httpapi/headers.go`), so the usual inline bootstrap script would be blocked in
production and every dark-mode user would get a white flash on load. Instead
`public/theme-bootstrap.js` is a separate file loaded synchronously ahead of the bundle. It repeats
a few constants from `src/app/theme/theme.ts`, and `theme-bootstrap.test.ts` fails if the two drift.

## Notes

- **TypeScript is pinned to `~6.0`**, not the current 7.x: `typescript-eslint` supports `<6.1.0`.
  Lift the pin when it supports 7.
- **No `eslint-plugin-jsx-a11y`**: it does not support ESLint 10 yet. Worth adding when it does.
- **`openapi-typescript` peers on `typescript@^5.x`** and this repo is on 6. `package.json` overrides
  that peer to the repo's own TypeScript; the generator uses the compiler API and runs fine on it.
  Drop the override when upstream widens the range.
