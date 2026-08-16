//! Connecting to a deployment: the part the OpenAPI document cannot say.
//!
//! Hand-written, like `src/lib.rs` and `Cargo.toml` and unlike everything under
//! `apis` and `models`. Anything that *can* be expressed in the OpenAPI document
//! belongs there instead — a helper here is a second description of the API, and
//! the point of this SDK is that there is only one.

use std::fmt;

use crate::apis::configuration::Configuration;

/// The prefix every operation in this crate hangs off.
///
/// The document declares its one server as the relative URL `/api/v1`, because
/// the SPA is served from the same origin as the API and an absolute URL would
/// pin every deployment to one host. A generated client cannot send a request to
/// a relative URL, so the prefix is applied by [`connect`] instead.
pub const API_PATH: &str = "/api/v1";

/// Why a base URL was refused.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ConnectError {
    /// `base_url` was empty or whitespace.
    EmptyBaseUrl,
    /// A token was supplied but was empty or whitespace. An empty string is not
    /// "no token": it would be sent as `Bearer ` and arrive as an anonymous
    /// call, failing as a 401 a long way from the mistake.
    EmptyToken,
}

impl fmt::Display for ConnectError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ConnectError::EmptyBaseUrl => write!(
                f,
                "base URL is empty; pass the deployment origin, such as https://blacklight.example.com"
            ),
            ConnectError::EmptyToken => {
                write!(f, "service token is empty; pass None for an anonymous client")
            }
        }
    }
}

impl std::error::Error for ConnectError {}

/// Build a [`Configuration`] for the Blacklight deployment at `base_url`.
///
/// `base_url` is the deployment's origin — `https://blacklight.example.com` —
/// with no API path on it; [`API_PATH`] is appended.
///
/// `token` is a service token, the `bl_<prefix>_<secret>` string shown once when
/// the token was created, sent as `Authorization: Bearer` on every request.
/// `None` builds an anonymous client, which reaches only the handful of
/// operations the document marks public.
///
/// The browser session cookie is deliberately not supported: a token can be
/// scoped and expired by an administrator, and driving the login and MFA
/// endpoints from a program to get a cookie instead is working around that.
///
/// The `reqwest::Client` is the default one. Replace
/// [`Configuration::client`] afterwards to set timeouts, a proxy or a custom
/// TLS configuration.
///
/// # Errors
///
/// Returns [`ConnectError`] if `base_url` is empty, or if `token` is `Some` but
/// empty.
pub fn connect(base_url: &str, token: Option<&str>) -> Result<Configuration, ConnectError> {
    let trimmed = base_url.trim();
    if trimmed.is_empty() {
        return Err(ConnectError::EmptyBaseUrl);
    }

    let bearer_access_token = match token {
        None => None,
        Some(token) if token.trim().is_empty() => return Err(ConnectError::EmptyToken),
        Some(token) => Some(token.to_owned()),
    };

    Ok(Configuration {
        // Trailing slashes on both halves would produce `//api/v1`, which some
        // reverse proxies redirect and others 404.
        base_path: format!("{}{}", trimmed.trim_end_matches('/'), API_PATH),
        bearer_access_token,
        ..Configuration::new()
    })
}
