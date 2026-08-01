package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// The commands in this file are registered, documented and inert. PLAN.md §6
// says popsctl does four things beyond migrations — create users, sync content,
// back up, render reports — and none of those features exists yet.
//
// They are here anyway so that the shape of the tool is visible from
// `popsctl --help` rather than discovered one milestone at a time, and so that
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

func newUserCommand() *cobra.Command {
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a user account (M1)",
		Long: "Creates a user account without going through the web interface — which is how\n" +
			"the first administrator of a new deployment gets in, and how the end-to-end\n" +
			"suite seeds the accounts its specs sign in as.",
		Args: noArgs,
		RunE: notImplemented("M1", "which builds identity and access"),
	}
	return group("user", "Manage user accounts (M1)", create)
}

func newContentCommand() *cobra.Command {
	sync := &cobra.Command{
		Use:   "sync",
		Short: "Install or refresh a content source (M2)",
		Long: "Installs or refreshes a content source — ATT&CK, Atomic Red Team, Sigma, CTID —\n" +
			"from the command line, for a deployment that cannot reach them from the\n" +
			"browser, or for seeding one before anybody signs in.",
		Args: noArgs,
		RunE: notImplemented("M2", "which builds the content registry"),
	}
	return group("content", "Manage content sources (M2)", sync)
}

func newBackupCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Take a consistent backup of the database and evidence (M7)",
		Long: "Takes a backup of the database and the evidence directory together, which is\n" +
			"the only combination that restores to anything useful.\n\n" +
			"Until this exists, docs/deploy.md documents the manual procedure: stop the\n" +
			"container, archive the data volume, start it again. The reason it says to stop\n" +
			"first is exactly what this command will remove.",
		Args: noArgs,
		RunE: notImplemented("M7", "which turns the manual procedure in docs/deploy.md into a command"),
	}
}

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
