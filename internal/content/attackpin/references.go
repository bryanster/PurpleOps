package attackpin

import "context"

// References counts engagement-side (and other non-content) references to an
// ATT&CK version pin. M3 implements this against engagement.attack_version and
// related tables; M2 ships [NopReferences] so delete can compile and always
// answer zero.
//
// DeleteVersion calls AttackVersion before cascading. A non-zero count becomes
// 409 with the count in the problem detail — never a silent cascade into war-
// room data.
type References interface {
	// AttackVersion returns how many external rows currently pin version.
	// version is already NormalizeVersion'd by the caller.
	AttackVersion(ctx context.Context, version string) (count int64, err error)
}

// NopReferences is the M2 stub: nothing outside content references a pin yet.
type NopReferences struct{}

// AttackVersion implements [References] with a constant zero.
func (NopReferences) AttackVersion(context.Context, string) (int64, error) {
	return 0, nil
}

// Compile-time check that the stub satisfies the extension point M3 will own.
var _ References = NopReferences{}
