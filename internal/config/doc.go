// Package config parses and validates the process configuration from the
// environment into a typed struct, so that no other package reads the
// environment directly (TestOnlyConfigReadsTheEnvironment enforces that).
//
// The contract is deliberately narrow:
//
//   - [Load] is called once, at startup, before anything else exists.
//   - It either returns a [Config] every field of which is usable, or a
//     [LoadError] listing every problem it found — never a partially valid
//     config, and never a silent fallback to an insecure default.
//   - A [Config] is a value. It is passed to constructors; nothing reaches
//     back into this package at runtime.
//
// Every variable is documented in .env.example at the repository root, which
// TestEnvExampleDocumentsEveryVariable keeps in step with the code.
package config
