package password

import (
	"maps"
	"slices"
)

// CommonPasswordsForTest returns the parsed contents of common_passwords.txt,
// in a stable order.
//
// The list is not part of the package's API — the only question anyone should
// ask it is [Validate] — but its contents are worth asserting on, so
// policy_test.go can check that no entry is added in a form the lookup could
// never match. Declared here so the export exists only in the test binary.
func CommonPasswordsForTest() []string {
	return slices.Sorted(maps.Keys(commonPasswords))
}
