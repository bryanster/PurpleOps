//! What these cover is the seam between the hand-written `deployment` module and
//! the generated client: where a request goes, what it carries, and how a
//! documented status arrives.
//!
//! They do not re-test the generator. Whether `list_engagements` serialises its
//! query parameters correctly is openapi-generator's business, and asserting it
//! here would mean writing the request builder out a second time by hand.

use blacklight::apis::engagements_api::list_engagements;
use blacklight::apis::system_api::get_health;
use blacklight::deployment::{connect, ConnectError, API_PATH};
use blacklight::models::HealthState;
use wiremock::matchers::{header, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn healthy() -> serde_json::Value {
    serde_json::json!({"status": "ok", "checks": {"db": "ok"}})
}

fn empty_page() -> serde_json::Value {
    serde_json::json!({"items": [], "nextCursor": null})
}

/// The reason `connect` exists: the document's one server is the relative URL
/// `/api/v1`, so a caller who set `base_path` to their origin would be talking
/// to the SPA's index.html.
#[tokio::test]
async fn connect_appends_the_api_path() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path(format!("{API_PATH}/healthz")))
        .respond_with(ResponseTemplate::new(200).set_body_json(healthy()))
        .expect(1)
        .mount(&server)
        .await;

    let configuration = connect(&server.uri(), None).expect("connect");
    get_health(&configuration).await.expect("get_health");
}

/// An operator's BLACKLIGHT_URL very often ends in a slash, and `//api/v1` is
/// redirected by some reverse proxies and 404ed by others.
#[tokio::test]
async fn connect_tolerates_a_trailing_slash() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path(format!("{API_PATH}/healthz")))
        .respond_with(ResponseTemplate::new(200).set_body_json(healthy()))
        .expect(1)
        .mount(&server)
        .await;

    let configuration = connect(&format!("{}///", server.uri()), None).expect("connect");
    get_health(&configuration).await.expect("get_health");
}

// `matches!` rather than `assert_eq!`: Configuration holds a reqwest::Client,
// which is not PartialEq, so the Ok side of the Result cannot be compared.
#[test]
fn connect_refuses_an_empty_base_url() {
    assert!(matches!(connect("   ", None), Err(ConnectError::EmptyBaseUrl)));
}

#[test]
fn connect_refuses_an_empty_token() {
    assert!(matches!(
        connect("https://blacklight.example.com", Some("  ")),
        Err(ConnectError::EmptyToken)
    ));
}

// An authenticated operation rather than /healthz: the generated code attaches
// the credential only where the document says one is required, so asserting
// this against a public endpoint would assert nothing.
#[tokio::test]
async fn a_token_is_sent_as_a_bearer_credential() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path(format!("{API_PATH}/engagements")))
        .and(header("authorization", "Bearer bl_abcd_secret"))
        .respond_with(ResponseTemplate::new(200).set_body_json(empty_page()))
        .expect(1)
        .mount(&server)
        .await;

    let configuration = connect(&server.uri(), Some("bl_abcd_secret")).expect("connect");
    list_engagements(&configuration, None, None, None)
        .await
        .expect("list_engagements");
}

#[tokio::test]
async fn no_token_sends_no_credential() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path(format!("{API_PATH}/engagements")))
        .respond_with(ResponseTemplate::new(200).set_body_json(empty_page()))
        .mount(&server)
        .await;

    let configuration = connect(&server.uri(), None).expect("connect");
    list_engagements(&configuration, None, None, None)
        .await
        .expect("list_engagements");

    let requests = server.received_requests().await.expect("recorded requests");
    assert_eq!(requests.len(), 1);
    assert!(requests[0].headers.get("authorization").is_none());
}

/// The whole argument for a generated client over hand-written requests: the
/// body comes back as the model the document describes, enum variants and all.
#[tokio::test]
async fn a_documented_response_parses_into_its_model() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path(format!("{API_PATH}/healthz")))
        .respond_with(ResponseTemplate::new(200).set_body_json(healthy()))
        .mount(&server)
        .await;

    let configuration = connect(&server.uri(), None).expect("connect");
    let health = get_health(&configuration).await.expect("get_health");

    assert_eq!(health.status, HealthState::Ok);
    assert_eq!(health.checks.db, HealthState::Ok);
}
