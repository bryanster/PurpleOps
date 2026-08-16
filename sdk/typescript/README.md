# blacklight-sdk (TypeScript / JavaScript)

A typed client for the Blacklight API, generated from
[`api/openapi.yaml`](../../api/openapi.yaml) with `openapi-typescript` and driven by
`openapi-fetch`. Every path, parameter, request body and response is checked at compile time: a path
that is not in the document, or a field the server does not send, is a `tsc` error rather than a 404
someone finds in production.

Ships as ESM with declarations. Node 20.19+ or 22.12+, or any modern browser — the only platform API
it uses is `fetch`.

`src/schema.ts` is generated. The hand-written files are `src/index.ts` and `src/errors.ts`.

## Connecting

```ts
import { createClient, unwrap } from 'blacklight-sdk'

const blacklight = createClient({
  baseUrl: 'https://blacklight.example.com',
  serviceToken: process.env.BLACKLIGHT_TOKEN,
})

const page = unwrap(await blacklight.GET('/engagements', { params: { query: { limit: 50 } } }))
for (const engagement of page.items) {
  console.log(engagement.name, engagement.status)
}
```

`createClient` takes the deployment's **origin** and appends `/api/v1` itself: the document declares
its one server as a relative URL, because the SPA is served from the same origin as the API.

The credential is a [service token](../../docs/api-tokens.md) — the `bl_<prefix>_<secret>` string
shown once when the token was created. Without one the client reaches only the operations the
document marks public. The browser session cookie is deliberately not supported here; the SPA is the
cookie's client and has its own in [`web/src/api`](../../web/src/api).

## Errors

By default a documented failure is **thrown** as an `ApiError`, so a script that forgets to check
does not carry on with data it does not have:

```ts
import { ApiError } from 'blacklight-sdk'

try {
  const engagement = unwrap(
    await blacklight.GET('/engagements/{engagementId}', {
      params: { path: { engagementId } },
    }),
  )
} catch (error) {
  if (error instanceof ApiError) {
    // `code` is a closed set and stable across releases. Branch on it, never on
    // the prose in `detail` and never on the status alone.
    if (error.code === 'not_found') return undefined
    if (error.code === 'rate_limited') await sleep((error.retryAfterSeconds ?? 60) * 1000)
  }
  throw error
}
```

`ApiError` carries `status`, `code`, `fieldErrors` (on `validation_failed`), `requestId` — which is
where a support conversation starts — `retryAfterSeconds`, and the whole `problem` document.

Two things are deliberately _not_ errors:

- **A non-2xx that is not a problem document.** `GET /healthz` answers `503` with the same `Health`
  body as its `200`, because the interesting part is which dependency is down. Distinguishing the
  two on `application/problem+json` is exactly what that media type is for.
- **Nothing, if you turn it off.** Pass `throwOnError: false` to get `openapi-fetch`'s
  `{ data, error }` back untouched, with `error` typed as the problem document. That is the better
  shape when a caller handles every failure explicitly.

## Streaming and downloads

The live event stream (`GET /events`) is `text/event-stream` — a long-lived connection rather than a
document, so there is no typed body to hand back. Use `EventSource` or `fetch` directly:

```ts
const url = new URL('/api/v1/events', 'https://blacklight.example.com')
url.searchParams.append('topics', `engagement.${engagementId}`)

const response = await fetch(url, { headers: { Authorization: `Bearer ${token}` } })
// ... then read response.body as a stream.
```

PDF, ZIP and evidence downloads are the same: `fetch` them and stream the body rather than expecting
a parsed one.

## Types

The generated schema is re-exported, so a caller can name what it passes around:

```ts
import type { components } from 'blacklight-sdk'

type Engagement = components['schemas']['Engagement']
type ProblemCode = components['schemas']['ProblemCode']
```

## Developing

From the repository root:

```
make generate     # rewrite src/schema.ts from api/openapi.yaml
make test-sdk     # run these tests, and the other three SDKs'
```

Inside `sdk/typescript`: `npm run build` emits `dist/`, `npm run lint` type-checks the sources _and_
the tests and checks formatting, `npm test` runs Vitest.
