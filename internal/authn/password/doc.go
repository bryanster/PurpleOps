// Package password hashes and checks the passwords of local accounts, and
// decides which passwords may be chosen in the first place.
//
// # One salt per hash, and no PASSWORD_SALT
//
// v1 mixed a single site-wide salt, read from the environment, into every
// password. A shared salt makes identical passwords hash identically for every
// user, which tells an attacker who holds the table which accounts to attack
// once — the property salting exists to destroy. Here each [Hash] draws its own
// random salt and writes it into the encoded string, so nothing about a
// password's storage lives in configuration and there is no value an operator
// can lose, leak or reuse.
//
// The encoding is the PHC string format Argon2's reference implementation
// uses:
//
//	$argon2id$v=19$m=65536,t=3,p=1$<salt>$<hash>
//
// The cost parameters travel with the hash rather than being implied by
// whatever the code does today. That is what lets [Verify] check a hash made
// years ago under weaker settings and tell the caller, through needsRehash,
// that it is now cheap enough to be worth replacing.
//
// # The plaintext has a type
//
// [Plaintext] exists so that a password cannot be printed by accident. It
// redacts itself for fmt, for log/slog and for encoding/json, so a struct
// holding one is safe to log and an error wrapping one is safe to return. Code
// that genuinely needs the characters says so out loud, with [Plaintext.Reveal].
//
// # Policy
//
// [Validate] applies the rules in one place: long enough, not absurdly long,
// not only whitespace, and not one of the passwords everybody picks. There are
// no composition rules — "must contain a symbol" pushes people towards
// Password1! and away from passphrases, and NIST SP 800-63B has advised
// against them since 2017.
package password
