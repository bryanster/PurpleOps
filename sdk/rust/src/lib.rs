//! Typed Rust client for the Blacklight API.
//!
//! Everything under [`apis`] and [`models`] is generated from
//! `api/openapi.yaml` by `make generate` — every path, parameter, request body
//! and response. Calling an operation the document does not describe, or reading
//! a field the server does not send, is a compile error rather than a surprise
//! at runtime.
//!
//! ```no_run
//! use blacklight::apis::engagements_api::list_engagements;
//! use blacklight::deployment::connect;
//!
//! # async fn example() -> Result<(), Box<dyn std::error::Error>> {
//! let configuration = connect("https://blacklight.example.com", Some("bl_abcd_secret"))?;
//!
//! let page = list_engagements(&configuration, None, Some(50), None).await?;
//! for engagement in page.items {
//!     println!("{} {:?}", engagement.name, engagement.status);
//! }
//! # Ok(())
//! # }
//! ```
//!
//! Operations are grouped by the document's tags: [`apis::engagements_api`],
//! [`apis::executions_api`], [`apis::reports_api`] and so on. Start from
//! [`deployment::connect`] for the [`apis::configuration::Configuration`] every
//! one of them takes.

// This file is hand-written. tools/generate-rust-sdk.sh copies only src/apis,
// src/models and .openapi-generator out of the generator's container, so the
// crate documentation above and the `deployment` module below are not lost at
// the next regeneration. The two `pub mod` lines are what the generator's own
// lib.rs declares; adding a module to src/ means adding it here.

#![allow(unused_imports)]
#![allow(clippy::too_many_arguments)]

extern crate reqwest;
extern crate serde;
extern crate serde_json;
extern crate serde_repr;
extern crate url;

pub mod apis;
pub mod deployment;
pub mod models;
