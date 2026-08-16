# blacklight-sdk (Python)

A typed Python client for the Blacklight API, generated from
[`api/openapi.yaml`](../../api/openapi.yaml). Everything under `blacklight/` except
`deployment.py` and `py.typed` is written by `make generate` — do not edit it.

Requires Python 3.11 or newer.

## Connecting

```python
import os

from blacklight.api.engagements.list_engagements import sync as list_engagements
from blacklight.deployment import connect
from blacklight.models import Problem

client = connect("https://blacklight.example.com", token=os.environ["BLACKLIGHT_TOKEN"])

page = list_engagements(client=client, limit=50)
if isinstance(page, Problem):
    raise SystemExit(f"{page.code}: {page.detail}")

for engagement in page.items:
    print(engagement.name, engagement.status)
```

`connect()` takes the deployment's **origin** and appends `/api/v1` itself; the
document declares its one server as a relative URL, because the SPA is served
from the same origin as the API. It is imported from `blacklight.deployment`
rather than from `blacklight` because the package's `__init__.py` is generated
and would lose the export at the next `make generate`.

The credential is a [service token](../../docs/api-tokens.md) — the
`bl_<prefix>_<secret>` string shown once when the token was created. Without one
the client reaches only the operations the document marks public. The browser
session cookie is deliberately not supported here: a token can be scoped and
expired by an administrator, and driving the login and MFA endpoints from a
script to get a cookie instead is working around that.

## Calling an operation

Every operation is a module under `blacklight.api.<tag>.<operation_id>`, with
four entry points:

| Function | Returns |
|---|---|
| `sync(...)` | the parsed success body, or `None` |
| `sync_detailed(...)` | a `Response` with `status_code`, `headers`, `content` and `parsed` |
| `asyncio(...)` | the same as `sync`, awaited |
| `asyncio_detailed(...)` | the same as `sync_detailed`, awaited |

Use the `_detailed` pair when the status code matters — `GET /healthz` answers
`503` with the same `Health` body as its `200`, and which one you got is the
whole point.

## Errors

Every failure this API describes is an RFC 9457 problem document, so a call's
return type is a union — `EngagementPage | Problem | None` above. A documented
failure comes back **parsed as a `Problem`**, not raised: `type`, `title`,
`status`, and a machine-readable `code`. Branch on `code`, never on `detail` and
never on the status alone.

`None` is the third case: a status the document does *not* describe. It is worth
turning that into an exception rather than a `None` that propagates:

```python
client = connect(url, token=token, raise_on_unexpected_status=True)
```

Transport failures — no server, TLS, a timeout — raise `httpx` exceptions, as
they would from `httpx` directly.

## What is not in here

Four things the generator cannot express, and what to do instead:

- **The live event stream** (`GET /events`, `text/event-stream`) is a
  long-lived connection rather than a document to buffer. Read it with `httpx`
  directly:

  ```python
  with httpx.stream(
      "GET",
      f"{url}/api/v1/events",
      params={"topics": [f"engagement.{engagement_id}"]},
      headers={"Authorization": f"Bearer {token}"},
      timeout=None,
  ) as response:
      for line in response.iter_lines():
          ...
  ```

- **SAML metadata** (`GET /auth/saml/metadata`) is XML, which the generator does
  not parse. The operation is generated; its body comes back as bytes.
- **PDF and ZIP downloads** come back as `File` objects, because the generator
  is told to read them as `application/octet-stream`
  (`api/codegen-sdk-python.yaml`).
- **The session cookie and CSRF**, as above — service tokens only.

## Developing

From the repository root:

```
make generate     # rewrite blacklight/ from api/openapi.yaml
make test-sdk     # run these tests, and the other three SDKs'
```

CI regenerates all four SDKs and fails if the result differs from what is
committed, so a change to the API document that is not regenerated does not
merge.
