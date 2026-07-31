// Package store owns persistence: the DuckDB connection pools, the serialized
// writer that funnels every write through a single goroutine, the embedded SQL
// migrations, and the repositories built on top of them.
//
// Repositories take their database handle as a constructor argument. There is
// no package-level database global — that is what produced the nil-handle panic
// class in v1 (PLAN.md §6).
//
// Implemented by M0B-003 (connections, writer) and M0B-004 (migrator).
package store
