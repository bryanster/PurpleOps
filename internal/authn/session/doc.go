// Package session issues, resolves, rotates and revokes browser sessions.
//
// It is the one place that decides whether a session is usable. The store
// (internal/store/identity) keeps the rows and interprets none of them; every
// question of expiry, idleness and revocation is answered here, so there is no
// second opinion to disagree with (M1-003).
//
// Three properties are worth stating out loud, because the rest of the package
// is built to keep them true:
//
//   - The token exists in the cookie and nowhere else. What is stored is a keyed
//     hash of it, so a copy of the database is not a set of live sessions, and
//     rotating PURPLEOPS_SESSION_SECRET invalidates every one of them.
//   - [Token] redacts itself the way a password does. Printing one, logging one
//     or serializing one produces a placeholder; reaching the characters takes
//     [Token.Reveal], which greps.
//   - Rotation keeps the session and replaces the token. The row's identifier,
//     creation time and absolute expiry survive, so rotating on every privilege
//     change costs a session nothing and gains the defence against session
//     fixation that PLAN.md §4 asks for.
package session
