// Package report turns an engagement into a deliverable: a registry of report
// blocks the user picks from, and the HTML and PDF renderers that lay them out.
//
// # Block registry
//
// Every report section is a registered [Definition] in a [Registry]. Blocks
// declare their identity ([ID]), parameter schema, data dependencies, and
// rendering behaviour. The registry is the single source of truth for which
// blocks exist and what they need — there is no undiscoverable block.
//
// # Data dependencies
//
// Blocks MUST NOT embed SQL aggregates or recompute rollups. Every data
// number comes from [internal/analytics] or domain repositories, injected
// through [RenderEnv]. The labels, heatmap ramp, and vocabulary that
// analytics and report blocks share are defined in [docs/analytics.md].
// A block that recomputes a rollup, or defines its own label for a value
// already labelled there, is a review rejection.
//
// # Renderer
//
// Each concrete block type implements [Renderer]. The report assembler
// (M6-009) calls every block's Render in order and concatenates the
// [HTMLFragment] results. That single HTML string is the draft preview,
// the published version body, the share view, and the PDF input — one
// rendering path, four consumers.
//
// # Parameters
//
// Block parameters are validated by [ValidateParams] against the block's
// [ParamSchema] — a simple JSON Schema object definition. Unknown keys and
// wrong types are rejected; omitted keys receive their declared defaults.
// The schema is the same object the API exposes to the builder UI, so the
// validation the server enforces is exactly what the picker displays.
//
// Implemented by M6.
package report
