package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
)

// print writes a command's result: the JSON document when --json was given,
// and whatever human takes when it was not.
//
// Every command goes through this rather than deciding for itself, so that
// "the result is on stdout, and with --json it is one parseable document" is
// true by construction rather than by review. human is not called at all in
// JSON mode, so a stray Fprintln in it cannot corrupt the output.
func (a *app) print(result any, human func(w *tabwriter.Writer)) error {
	if a.jsonOut {
		// Indented: still exactly one document for jq, and readable when
		// somebody runs it without a pipe to see what the fields are called.
		encoder := json.NewEncoder(a.out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("writing the result: %w", err)
		}
		return nil
	}

	// Two spaces between columns, so a value that is much wider than its
	// heading does not push everything else off the terminal.
	w := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	human(w)
	if err := w.Flush(); err != nil {
		return fmt.Errorf("writing the result: %w", err)
	}
	return nil
}

// plural renders a count with its noun: "1 migration", "2 migrations". Only
// regular plurals, which is all this tool counts.
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// byteUnits are binary multiples, which is what `ls -lh` and every filesystem
// tool report and therefore what an operator will compare this against.
var byteUnits = []struct {
	suffix string
	size   float64
}{
	{"GiB", 1 << 30},
	{"MiB", 1 << 20},
	{"KiB", 1 << 10},
}

// humanBytes renders a file size for a person. The exact byte count is in the
// JSON output; this is the one that answers "is it big?" at a glance.
func humanBytes(n int64) string {
	for _, unit := range byteUnits {
		if float64(n) >= unit.size {
			return fmt.Sprintf("%.1f %s", float64(n)/unit.size, unit.suffix)
		}
	}
	return fmt.Sprintf("%d B", n)
}
