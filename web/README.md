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

| Command              | What it does                                 |
| -------------------- | -------------------------------------------- |
| `npm run dev`        | Vite dev server with the `/api` proxy        |
| `npm run build`      | Type-check, then build to `dist/`            |
| `npm run lint`       | `tsc --noEmit`, ESLint, and a Prettier check |
| `npm run format`     | Rewrite files with Prettier                  |
| `npm test`           | Vitest, once                                 |
| `npm run test:watch` | Vitest, watching                             |

`make lint test build` at the repo root runs the Go and web halves together, which is what CI does.

## Layout

Organised by feature, not by file type — the domain in `PLAN.md` §2 is large enough that a single
`components/` directory stops scaling around M3.

```
public/theme-bootstrap.js   Sets the theme before first paint (see "Theming")
src/app/                    The application shell: nav, layout, theme, error boundary, routes
src/components/ui/          Vendored shadcn/ui primitives — see below
src/features/<feature>/     One directory per feature. Screens, and the data they need
src/lib/                    Small shared helpers with no feature knowledge
src/test/                   Vitest setup and shared test utilities
```

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
- **`src/features/system/api.ts` and `src/lib/use-async.ts` are temporary.** M0B-009 replaces them
  with a client generated from `api/openapi.yaml` and TanStack Query. Nothing outside those two
  files' own directories imports them, so that replacement stays local.
