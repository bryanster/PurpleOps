# blacklight (Rust)

A typed async Rust client for the Blacklight API, generated from
[`api/openapi.yaml`](../../api/openapi.yaml). Built on `reqwest` and `serde`; every operation is an
`async fn` taking a `&Configuration`.

`src/apis/` and `src/models/` are generated. The hand-written files are `Cargo.toml`, `src/lib.rs`
and `src/deployment.rs`.

```toml
[dependencies]
blacklight = { git = "https://github.com/bryanster/blacklight", path = "sdk/rust" }
tokio = { version = "1", features = ["macros", "rt-multi-thread"] }
```

TLS comes from `native-tls` by default; `default-features = false, features = ["rustls"]` links
nothing from the platform.

## Connecting

```rust
use blacklight::apis::engagements_api::list_engagements;
use blacklight::deployment::connect;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let configuration = connect(
        "https://blacklight.example.com",
        Some(&std::env::var("BLACKLIGHT_TOKEN")?),
    )?;

    let page = list_engagements(&configuration, None, Some(50), None).await?;
    for engagement in page.items {
        println!("{} {:?}", engagement.name, engagement.status);
    }

    Ok(())
}
```

`connect` takes the deployment's **origin** and appends `/api/v1` itself: the document declares its
one server as a relative URL, because the SPA is served from the same origin as the API.

The credential is a [service token](../../docs/api-tokens.md) — the `bl_<prefix>_<secret>` string
shown once when the token was created, sent as `Authorization: Bearer` on the operations that
require one. Pass `None` for an anonymous client, which reaches only the operations the document
marks public. The browser session cookie is deliberately not supported here; see
[`docs/sdk.md`](../../docs/sdk.md).

`connect` returns the default `reqwest::Client`. Replace `configuration.client` to set timeouts, a
proxy or a custom TLS configuration.

## Errors

Every call returns `Result<T, Error<SomethingError>>`, and the two failure modes are distinct:

```rust
use blacklight::apis::engagements_api::{get_engagement, GetEngagementError};
use blacklight::apis::Error;

match get_engagement(&configuration, &engagement_id).await {
    Ok(engagement) => use_it(engagement),
    // The server answered and said no. `entity` is the typed problem document.
    Err(Error::ResponseError(content)) => match content.entity {
        Some(GetEngagementError::Status404(problem)) => {
            // Branch on `code`, never on the prose in `detail` and never on the
            // status alone.
            eprintln!("{:?}: {:?}", problem.code, problem.detail);
        }
        _ => eprintln!("HTTP {}", content.status),
    },
    // The exchange itself failed: no server, TLS, a timeout.
    Err(other) => return Err(other.into()),
}
```

## Streaming and downloads

The live event stream (`GET /events`) is `text/event-stream` — a long-lived connection rather than a
document — and the generated function would buffer it. Use `reqwest` directly:

```rust
let response = configuration
    .client
    .get(format!("{}/events", configuration.base_path))
    .query(&[("topics", format!("engagement.{engagement_id}"))])
    .bearer_auth(token)
    .send()
    .await?;
// ... then read response.bytes_stream().
```

PDF, ZIP and evidence downloads are the same when the file is large: the generated call reads the
whole body into memory.

## Developing

From the repository root:

```
make generate     # rewrite src/apis and src/models from api/openapi.yaml
make test-sdk     # run these tests, and the other three SDKs'
```

Generation needs Docker and no Rust toolchain — the generator is a container image pinned by digest
(`tools/generate-rust-sdk.sh`). Testing needs `cargo`, which the devcontainer installs.

`Cargo.lock` is committed. Cargo ignores a library's lock file when the crate is used as a
dependency, so it constrains nobody downstream; here it is what makes `make test-sdk` and the CI job
compile the same dependency versions, and it is the cache key in the workflow.
