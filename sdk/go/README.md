# blacklight (Go)

A typed Go client for the Blacklight API, generated from
[`api/openapi.yaml`](../../api/openapi.yaml) by the same `oapi-codegen`, at the same version, that
generates the server — so both sides of the wire agree about what a nullable field or an enum
constant is, because one program decided both.

```
go get github.com/bryanster/blacklight/sdk/go
```

`sdk/go` is a module of its own, nested inside the repository's. `go get` of a client for a JSON API
must not drag DuckDB (cgo, and a C++ static archive), chromedp and the SSO stacks into your build,
so its requirements are exactly the three the generated code imports.

`client.gen.go` is generated. The only hand-written file is `blacklight.go`.

## Connecting

```go
package main

import (
	"context"
	"log"
	"os"

	blacklight "github.com/bryanster/blacklight/sdk/go"
)

func main() {
	client, err := blacklight.New(
		"https://blacklight.example.com",
		blacklight.WithServiceToken(os.Getenv("BLACKLIGHT_TOKEN")),
	)
	if err != nil {
		log.Fatal(err)
	}

	limit := blacklight.Limit(50)
	page, err := client.ListEngagementsWithResponse(context.Background(), &blacklight.ListEngagementsParams{
		Limit: &limit,
	})
	if err != nil {
		log.Fatal(err) // the request never completed
	}
	if page.JSON200 == nil {
		log.Fatalf("list engagements: %s", page.Status())
	}

	for _, engagement := range page.JSON200.Items {
		log.Printf("%s %s", engagement.Name, engagement.Status)
	}
}
```

[`New`] takes the deployment's **origin** and appends `/api/v1` itself: the document declares its
one server as a relative URL, because the SPA is served from the same origin as the API.

The credential is a [service token](../../docs/api-tokens.md) — the `bl_<prefix>_<secret>` string
shown once when the token was created. Without one the client reaches only the operations the
document marks public. The browser session cookie is deliberately not supported here; see
[`docs/sdk.md`](../../docs/sdk.md).

## Responses

Every operation has two methods. `ListEngagements` returns an `*http.Response` and leaves the body
to you; **`ListEngagementsWithResponse` is the one to use** — it parses each documented status code
into its own field:

```go
// engagementID is a blacklight.EngagementId, which is a uuid.UUID.
got, err := client.GetEngagementWithResponse(ctx, engagementID)
switch {
case err != nil:
	// The request did not complete: no server, TLS, a cancelled context.
case got.JSON200 != nil:
	use(got.JSON200)
case got.ApplicationproblemJSON404 != nil:
	// An RFC 9457 problem document, typed. Branch on Code, never on the
	// prose in Detail and never on the status alone.
	log.Printf("not found: %s", got.ApplicationproblemJSON404.Detail)
}
```

`err` and a problem document are different things: the first means the exchange failed, the second
means the server answered and said no.

`GET /healthz` is the reason the fields are per status rather than "success or error": it answers
`503` with the same `Health` body as its `200`, and which one you got is the whole point.

## Streaming and downloads

The live event stream (`GET /events`) is `text/event-stream` — a long-lived
connection rather than a document, so the typed wrapper has nothing useful to give you. Use the
plain method and read the body:

```go
topics := []string{"engagement." + engagementID.String()}
resp, err := client.SubscribeEvents(ctx, &blacklight.SubscribeEventsParams{Topics: &topics})
// ... then read resp.Body incrementally. No *WithResponse: there is no final
// body to parse, and calling it would block until the stream closed.
```

PDF, ZIP and evidence downloads work the same way: call the plain method and stream `resp.Body`
rather than buffering a typed one.

## Options

`New` takes any `ClientOption` the generated client accepts:

```go
client, err := blacklight.New(url,
	blacklight.WithServiceToken(token),
	blacklight.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
	blacklight.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Set("User-Agent", "my-tool/1.0")
		return nil
	}),
)
```

## Developing

From the repository root:

```
make generate     # rewrite client.gen.go from api/openapi.yaml
make test-sdk     # run these tests, and the other three SDKs'
```
