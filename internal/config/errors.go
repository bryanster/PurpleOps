package config

import (
	"fmt"
	"strings"
)

// FieldError is a problem with exactly one environment variable. It always
// names the variable, because "invalid config" tells an operator nothing about
// which of ten values to go and fix.
type FieldError struct {
	// Name is the environment variable, e.g. "PURPLEOPS_BASE_URL".
	Name string
	// Value is the offending value, echoed back so the operator can see what
	// the process actually received (leading whitespace, a stray quote from a
	// compose file). It is empty when the value must not be echoed — see
	// binding.sensitive — and when there was no value at all.
	Value string
	// Msg states the requirement, phrased to follow the variable name:
	// "must be an absolute URL".
	Msg string
}

func (e *FieldError) Error() string {
	if e.Value == "" {
		return e.Name + ": " + e.Msg
	}
	return fmt.Sprintf("%s: %s, got %q", e.Name, e.Msg, e.Value)
}

// LoadError is every problem [Load] found, not just the first. Fixing a
// deployment one restart per mistake is the failure mode this exists to avoid.
type LoadError struct {
	Errs []error
}

func (e *LoadError) Error() string {
	if len(e.Errs) == 1 {
		return "invalid configuration: " + e.Errs[0].Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "invalid configuration (%d problems):", len(e.Errs))
	for _, err := range e.Errs {
		b.WriteString("\n  - ")
		b.WriteString(err.Error())
	}
	return b.String()
}

// Unwrap exposes the individual problems to errors.Is and errors.As.
func (e *LoadError) Unwrap() []error { return e.Errs }
