# src/api — the generated client

Nothing in `web/src` hand-writes a URL, a request body type or a response type. `schema.d.ts` is
generated from `../../../api/openapi.yaml`, `client.ts` is the only module that knows what an API
URL looks like, and features import hooks.

If the spec changes, `tsc` names the component that broke. That is the whole point of this
directory — see [`docs/api.md`](../../../docs/api.md).

| File                 | What it is                                                                                                      |
| -------------------- | --------------------------------------------------------------------------------------------------------------- |
| `schema.d.ts`        | **Generated** by `npm run generate` (`make generate`). Never edit; CI compares it against a fresh run           |
| `client.ts`          | The `openapi-fetch` instance, the base URL, and the middleware that turns a problem document into an `ApiError` |
| `errors.ts`          | `ApiError` and the problem-document types. Callers switch on `error.code`                                       |
| `query-client.ts`    | The QueryClient and its deliberately-chosen defaults                                                            |
| `query-provider.tsx` | Wires the global 401 and 5xx handling into the app                                                              |

## Calling an endpoint

Never from a component. A feature directory owns a `queries.ts` that exports `queryOptions()`
factories and a hook per resource; components import the hook:

```ts
export function versionQueryOptions() {
  return queryOptions({
    queryKey: systemKeys.version(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/version', { signal })),
  })
}

export function useVersion() {
  return useQuery(versionQueryOptions())
}
```

`queryOptions()` rather than an inline `useQuery({...})` keeps the key and the fetcher in one
object, which is what makes `prefetchQuery`, `setQueryData` and `invalidateQueries` possible without
repeating either.

## Query keys

A key is the path to the thing, from the outside in, mirroring the URL:

```ts
;['engagements'] // the collection
;['engagements', id] // one engagement
;['engagements', id, 'executions'] // a sub-collection
;['engagements', id, 'executions', { round: 2 }] // …narrowed by parameters
```

Rules that follow from that shape:

- **Segments are literals; the last element is an object** when there are query parameters. Keeping
  filters in one object means `invalidateQueries({ queryKey: ['engagements', id, 'executions'] })`
  invalidates every filtered view of them, because prefix matching stops at the object.
- **One `<feature>Keys` object per feature**, exported from its `queries.ts` and built from a single
  root (see `features/system/queries.ts`). Hand-writing an array at a call site is how two spellings
  of the same key end up in the cache — and a cache miss looks like a slow screen, not a bug.
- **Parameters that change the answer go in the key.** A key that omits one serves the wrong round's
  data to whoever asks second.

## Errors

`client.ts` throws an `ApiError` for every problem document, so a query's `error` is typed and
`error.code` is the stable identifier to branch on. The exception is a non-2xx the spec documents a
real body for — `GET /healthz` answers 503 with a `Health` — which is returned, not thrown.

Two failures are handled globally in `query-provider.tsx` and must not be repeated per screen: a
**401** navigates to the login route, and a **5xx** raises one toast. Everything else belongs to the
screen that asked, which knows what the request was for.

Queries **never retry a 4xx** (`shouldRetryQuery`). A 404 is an answer, and asking three times does
not change it.
