// Package identity stores who may use this installation: the people, the login
// methods attached to them, their live sessions, and their place in an
// engagement.
//
// It is storage and nothing else. Hashing a password (M1-002), deciding whether
// a session is still live (M1-003), enrolling a second factor (M1-006) and
// answering "may they" (M1-012) all sit above it. A repository here writes what
// it is handed and reads it back; the only judgement it makes is about rows
// that are absent, which it reports as [apierr.NotFound] so that a handler does
// not have to translate sql.ErrNoRows for itself.
//
// # Two levels of role, never one
//
// PLAN.md §4 keeps platform authority and engagement authority apart, because
// v1 had one blurred level and leaked permissions through it. [User.PlatformRole]
// says what somebody may do to the installation; [Membership.Role] says what
// they may do inside one engagement. They are different types over different
// vocabularies, so neither can be passed where the other is expected, and the
// schema constrains both (see 0002_identity.sql).
//
// Both types are internal/authz's, not this package's (M1-012). Where a role is
// stored is a detail; what it means is the policy's business, and there is
// exactly one place that says so — a test fails the build if a role string
// appears anywhere else in the Go tree. That is the direct fix for v1's two
// contradictory definitions of "blue".
//
// # No repository owns a database
//
// Every repository is constructed with a [DB] — the store's serialized writer
// and pooled reader, and nothing wider. None of them holds a *sql.DB, so none
// of them can open a transaction outside [store.DB.Write], which is what keeps
// the single-writer rule from being a convention. There is no package-level
// handle to reach for either (PLAN.md §6).
//
// # Emails are compared by the database
//
// Every statement that reads or writes an email normalizes it in SQL, with
// lower(trim(...)). One definition, in one place, used by the uniqueness
// constraint and by every lookup alike — so Go and DuckDB cannot come to
// different conclusions about whether two addresses are the same, and the
// CHECK constraint that ties the two columns together cannot be tripped by a
// caller that normalized differently.
package identity
