// Package analytics answers aggregate questions — coverage, detection rates,
// mean-time-to-detect, findings burndown, cross-engagement compare — with
// DuckDB queries, keeping the analytical SQL out of the repositories that
// serve individual entities.
//
// # One source, two consumers
//
// Every number the dashboard (M5-013) and every renderer block (M6) report is
// the same number, from the same query. There is no second aggregation path.
//
// # DuckDB is the documented exception
//
// This package is exempt from the ANSI-only rule (README § Conventions).
// DuckDB-specific constructs used and their porting costs:
//
//   - PERCENTILE_CONT / PERCENTILE_DISC — Standard SQL:2003, but DuckDB
//     provides a SIMD implementation behind them. An ANSI database would
//     need an application-side post-pass or a window-function approximation.
//   - APPROX_QUANTILE — A DuckDB extension for t-digest-based approximate
//     percentiles. Exists for faster MTTD on large datasets. The standard
//     fallback is PERCENTILE_CONT, at higher cost.
//   - json_array_length / json_extract — DuckDB JSON functions. In PostgreSQL
//     the equivalents are jsonb_array_length / jsonb_path_query; in SQLite
//     they are json_array_length / json_extract — same names, different
//     behaviour around NULL and malformed input.
//   - list / list_distinct / list_sort — DuckDB list aggregates. Standard SQL
//     has ARRAY_AGG and the array is often opaque; building the deduplicated,
//     sorted list to hash for a stable diff requires a window in portable
//     SQL.
//   - unnest — EXISTS in PostgreSQL and DuckDB but not in SQLite (the
//     nearest portable target); a portable equivalent is a join against a
//     generate_series or a recursive CTE.
//
// # Rollup rule
//
// Every exported rollup (M5-004…M5-008) takes exactly one [Scope]. A rollup
// that took a bare engagement id would compile — Go cannot see which columns a
// query reads — and would leak a blind engagement's totals to blue while
// looking correct in this package's own tests. One [Scope] per rollup, in
// every signature, enforced by review.
//
// # Service layer
//
// This package answers questions and returns structs. Who may ask is
// [authz.Can]'s business, in the one middleware that asks it. There is
// deliberately no Service, no HTTP dependency, and no import of httpapi.
//
// # Reads only
//
// Analytics queries use the read pool and never call store.Write — they
// take no locks and block no war room.
package analytics
