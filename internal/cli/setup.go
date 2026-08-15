package cli

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/setup"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/settings"
)

// `blctl setup` is the command line's half of the first-run wizard: read
// whether an installation has been through it, mark it done, or put it back.
//
// Two audiences. A provisioning run that creates the first administrator with
// `blctl user create` and installs content with `blctl content sync` has
// already done everything the wizard asks, and `setup complete` stops the
// browser opening on a screen with nothing left to decide. And the end-to-end
// suite, whose specs are about other screens and which seeds this the same way
// it seeds an account.
//
// `setup reset` is deliberately not an endpoint. Putting an installation back
// into first-run state is an operator's decision made at the machine, and an
// endpoint for it would be a button somebody clicks by accident.

func newSetupCommand(a *app) *cobra.Command {
	return group("setup", "Inspect and control first-run setup",
		newSetupStatusCommand(a),
		newSetupCompleteCommand(a),
		newSetupResetCommand(a))
}

// setupResult is what `setup --json` prints, in the same shape `GET /setup`
// answers with, so a script does not need two readers.
type setupResult struct {
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
	CompletedBy string `json:"completedBy,omitempty"`
}

func toSetupResult(state setup.State) setupResult {
	out := setupResult{Completed: state.Completed, CompletedBy: state.CompletedBy}
	if !state.CompletedAt.IsZero() {
		out.CompletedAt = state.CompletedAt.Format(time.RFC3339)
	}
	return out
}

func (r setupResult) print(w *tabwriter.Writer) {
	if !r.Completed {
		fmt.Fprintf(w, "This installation has not been set up.\n\n")
		fmt.Fprintf(w, "completed\tno\n")
		fmt.Fprintf(w, "\nThe next administrator to sign in lands on the setup wizard,\n")
		fmt.Fprintf(w, "which asks which MITRE ATT&CK version to install.\n")
		return
	}
	fmt.Fprintf(w, "This installation has been set up.\n\n")
	fmt.Fprintf(w, "completed\tyes\n")
	fmt.Fprintf(w, "completed at\t%s\n", r.CompletedAt)
	if r.CompletedBy == "" {
		fmt.Fprintf(w, "completed by\t(not a person — the command line, or a provisioning run)\n")
		return
	}
	fmt.Fprintf(w, "completed by\t%s\n", r.CompletedBy)
}

func newSetupStatusCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report whether first-run setup has been finished",
		Long: "Prints whether anybody has been through the setup wizard, when, and who.\n\n" +
			"\"Completed\" records a decision rather than an outcome: it does not mean\n" +
			"content is installed. `blctl content status` answers that one.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				if err := a.requireMigrated(ctx, db); err != nil {
					return err
				}
				state, err := setupState(ctx, db)
				if err != nil {
					return err
				}
				result := toSetupResult(state)
				return a.print(result, result.print)
			})
		},
	}
}

func newSetupCompleteCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "complete",
		Short: "Mark first-run setup as finished",
		Long: "Marks this installation as set up, so that the next administrator to sign in\n" +
			"lands on the product rather than on the setup wizard.\n\n" +
			"For a provisioning run that has already created an administrator and installed\n" +
			"content, and for test harnesses seeding an installation that is past its first\n" +
			"boot. It installs nothing itself.\n\n" +
			"Running it twice keeps the first timestamp: when an installation was set up is\n" +
			"not something a repeated command should move.\n\n" +
			"The row records no user, because nobody signed in to write it.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				if err := a.requireMigrated(ctx, db); err != nil {
					return err
				}
				svc, err := setup.New(setup.Deps{Settings: settings.New(db)})
				if err != nil {
					return err
				}
				// No actor: this is the command line, and an empty subject
				// stores NULL rather than inventing a user who did it.
				state, err := svc.Complete(ctx, authn.Subject{})
				if err != nil {
					return err
				}
				result := toSetupResult(state)
				return a.print(result, result.print)
			})
		},
	}
}

func newSetupResetCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Put this installation back into first-run state",
		Long: "Forgets that the setup wizard was finished, so the next administrator to sign\n" +
			"in is shown it again.\n\n" +
			"Nothing else changes: users, engagements, and every installed content version\n" +
			"stay exactly where they are. This clears one row in the platform settings, and\n" +
			"the wizard it brings back can be skipped in one click.\n\n" +
			"There is no endpoint for this on purpose. An installation being put back into\n" +
			"first-run state is a decision made at the machine.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				if err := a.requireMigrated(ctx, db); err != nil {
					return err
				}
				if err := settings.New(db).Delete(ctx, setup.KeyCompletedAt); err != nil {
					return err
				}
				state, err := setupState(ctx, db)
				if err != nil {
					return err
				}
				result := toSetupResult(state)
				return a.print(result, result.print)
			})
		},
	}
}

func setupState(ctx context.Context, db *store.DB) (setup.State, error) {
	svc, err := setup.New(setup.Deps{Settings: settings.New(db)})
	if err != nil {
		return setup.State{}, err
	}
	return svc.State(ctx)
}
