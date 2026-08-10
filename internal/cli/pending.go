package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// The commands in this file are registered, documented and inert. PLAN.md §6
// says blctl does four things beyond migrations — create users, sync content,
// back up, render reports — and backup shipped in M7 (backup.go). The report
// render command is still pending.
//
// They are here anyway so that the shape of the tool is visible from
// `blctl --help` rather than discovered one milestone at a time, and so that
// the milestone that builds a feature adds a RunE to a command that already
// exists instead of arguing about where it goes. Each says which milestone that
// is, because "not implemented" without a date is indistinguishable from
// abandoned.
//
// None of them takes the app the real commands do, which is the shortest
// statement of what they are: nothing to configure, nothing to open.

// notImplemented builds the RunE of a command whose feature is still ahead of
// us. It exits 1, not 0: a script that reaches one of these did not get what it
// asked for, and the exit code is the only part of that a script can see.
func notImplemented(milestone, summary string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		return fmt.Errorf("`%s` is not implemented in this milestone; it arrives in %s, %s",
			cmd.CommandPath(), milestone, summary)
	}
}

// content commands live in content.go (M2-002 / M2-003).


func newReportCommand() *cobra.Command {
	render := &cobra.Command{
		Use:   "render",
		Short: "Render a report to HTML or PDF (M6)",
		Long: "Renders a saved report to HTML or PDF without a browser session, for a\n" +
			"scheduled job or a pipeline that files the result somewhere.",
		Args: noArgs,
		RunE: notImplemented("M6", "which builds reporting"),
	}
	return group("report", "Render reports (M6)", render)
}
