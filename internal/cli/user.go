package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/settings"
)

// `blctl user create` is how the first administrator of a deployment gets in:
// there is no sign-up, and the web interface has nothing to offer somebody who
// cannot sign in. It is also what the end-to-end suite seeds its accounts with.

func newUserCommand(a *app) *cobra.Command {
	return group("user", "Manage user accounts",
		newUserCreateCommand(a),
		newUserResetMFACommand(a))
}

// userResult is what `user create --json` prints. It carries no password and no
// hash: the first is gone by the time this is built, and the second is not
// something to put on a terminal that scrolls back.
type userResult struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	DisplayName  string `json:"displayName"`
	PlatformRole string `json:"platformRole"`
	Status       string `json:"status"`
}

func newUserCreateCommand(a *app) *cobra.Command {
	var (
		email string
		name  string
		admin bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a user account",
		Long: "Creates a user account without going through the web interface — which is how\n" +
			"the first administrator of a new deployment gets in, and how the end-to-end\n" +
			"suite seeds the accounts its specs sign in as.\n\n" +
			"The password is asked for on the terminal and not echoed. When stdin is not a\n" +
			"terminal it is read from there instead, so a script can pipe one in:\n\n" +
			"    printf '%s' \"$PASSWORD\" | blctl user create --email a@b.c --name Alice --admin\n\n" +
			"It is never taken from a flag or an environment variable: both end up in shell\n" +
			"history, in `ps`, and in whatever collects the logs.\n\n" +
			"The account is created active, with a local password login. The database must\n" +
			"already be migrated — run `blctl migrate up` first, or start the server once.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch {
			case strings.TrimSpace(email) == "":
				return usagef(cmd, "--email is required")
			case strings.TrimSpace(name) == "":
				return usagef(cmd, "--name is required")
			}

			plaintext, err := a.readPassword()
			if err != nil {
				return err
			}
			if err := password.Validate("password", plaintext); err != nil {
				return policyFailure(err)
			}
			hash, err := password.Hash(plaintext)
			if err != nil {
				return err
			}

			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				if err := a.requireMigrated(ctx, db); err != nil {
					return err
				}
				created, err := createUser(ctx, db, identity.NewUser{
					Email:        email,
					DisplayName:  name,
					PasswordHash: hash,
					PlatformRole: platformRole(admin),
					Status:       identity.StatusActive,
				})
				if err != nil {
					return err
				}

				result := userResult{
					ID:           created.ID,
					Email:        created.Email,
					DisplayName:  created.DisplayName,
					PlatformRole: string(created.PlatformRole),
					Status:       string(created.Status),
				}
				return a.print(result, func(w *tabwriter.Writer) {
					fmt.Fprintf(w, "Created %s (%s).\n\n", result.Email, result.PlatformRole)
					fmt.Fprintf(w, "id\t%s\n", result.ID)
					fmt.Fprintf(w, "email\t%s\n", result.Email)
					fmt.Fprintf(w, "name\t%s\n", result.DisplayName)
					fmt.Fprintf(w, "platform role\t%s\n", result.PlatformRole)
					fmt.Fprintf(w, "status\t%s\n", result.Status)
				})
			})
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&email, "email", "", "email address the account signs in with (required)")
	flags.StringVar(&name, "name", "", "name to show for this person (required)")
	flags.BoolVar(&admin, "admin", false,
		"make them a platform administrator, who may manage users, content and every engagement")
	return cmd
}

// `blctl user reset-mfa` is the break-glass path of M1-007: the only way back
// into an account whose second factor is gone along with the codes that were
// supposed to cover exactly that. It needs the database file and therefore the
// host, which is the access control — there is no API for this, deliberately,
// because an endpoint that strips somebody's second factor is an endpoint worth
// attacking.

// resetMFAResult is what `user reset-mfa --json` prints: who it was done to,
// and what was actually removed. The counts are read before the delete, so a
// script can tell "there was nothing to remove" from "seven working codes are
// now gone".
type resetMFAResult struct {
	ID    string `json:"id"`
	Email string `json:"email"`

	AuthenticatorRemoved bool `json:"authenticatorRemoved"`
	RecoveryCodesRemoved int  `json:"recoveryCodesRemoved"`
	ChallengesDiscarded  bool `json:"challengesDiscarded"`

	// MFAStillRequired is whether a second factor is *still* required of this
	// account after the reset — by the platform policy or by their own flag
	// (M1-008). It decides which of two opposite warnings is printed, so it is
	// read here rather than left for an operator to work out: this command
	// makes a password sufficient only when the answer is false.
	MFAStillRequired bool `json:"mfaStillRequired"`
}

func newUserResetMFACommand(a *app) *cobra.Command {
	var email string

	cmd := &cobra.Command{
		Use:   "reset-mfa",
		Short: "Remove a user's second factor and recovery codes",
		Long: "Removes an account's authenticator enrolment, every recovery code it holds and\n" +
			"any half-finished sign-in, so that whoever lost their phone can get back in.\n\n" +
			"What that means depends on whether a second factor is still required of them.\n" +
			"If nothing requires one, the account signs in with its password alone until\n" +
			"somebody enrols an authenticator again — genuinely a lock being broken, and the\n" +
			"output says so. If the platform policy or their own flag requires one, their\n" +
			"next sign-in can do exactly one thing: enrol a new authenticator. This command\n" +
			"does not change that, and does not touch the policy or the flag.\n\n" +
			"It exists because the alternative in a self-hosted, single-tenant tool is worse:\n" +
			"a lost phone belonging to the only administrator would otherwise mean editing\n" +
			"the database by hand or reinstalling.\n\n" +
			"Reach for recovery codes first: they were issued when the authenticator was\n" +
			"enrolled and they need nobody's help. This is for when those are gone too.\n\n" +
			"It does not touch the password and does not sign anybody out. An account that\n" +
			"had no second factor is not an error — it comes back saying nothing was removed.\n\n" +
			"The server holds the database file open, so stop it first.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(email) == "" {
				return usagef(cmd, "--email is required")
			}

			cfg, err := a.settings()
			if err != nil {
				return err
			}

			return a.withStore(cmd.Context(), func(ctx context.Context, db *store.DB) error {
				if err := a.requireMigrated(ctx, db); err != nil {
					return err
				}

				user, err := identity.NewUsers(db).ByEmail(ctx, email)
				if errors.Is(err, apierr.ErrNotFound) {
					// Named, unlike the API's answer to the same question. An
					// operator at a terminal already knows who they meant, and
					// there is nobody to enumerate accounts for.
					return fmt.Errorf("no account holds %s; "+
						"addresses are matched without regard to case", email)
				}
				if err != nil {
					return err
				}

				result, err := resetMFA(ctx, db, user)
				if err != nil {
					return err
				}

				// To the log as well as to stdout. Removing somebody's second
				// factor is the security-relevant event M1-007 wants in M1-015's
				// activity log; until that table exists this line is the record,
				// and it is written at warn so it stands out in whatever
				// collects it.
				a.logger(cfg).WarnContext(ctx, "second factor reset from the command line",
					slog.String("user_id", result.ID),
					slog.String("email", result.Email),
					slog.Bool("authenticator_removed", result.AuthenticatorRemoved),
					slog.Int("recovery_codes_removed", result.RecoveryCodesRemoved))

				return a.print(result, func(w *tabwriter.Writer) { writeResetMFA(w, result) })
			})
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "email address of the account to reset (required)")
	return cmd
}

// resetMFA removes every piece of second-factor state a user has, and reports
// what was there.
//
// The counts are taken first, in the order the deletes happen, so the report
// describes what this command removed rather than what is left. Three separate
// writes rather than one transaction: they are three repositories, and a failure
// between them leaves *less* second-factor state than before, which is the
// direction this command is already going in. Re-running it finishes the job.
func resetMFA(ctx context.Context, db *store.DB, user identity.User) (resetMFAResult, error) {
	result := resetMFAResult{ID: user.ID, Email: user.Email}

	// Read before anything is removed, because it decides what the operator is
	// told and nothing here changes the answer: this command does not touch
	// mfa_enforced or the platform policy, on purpose (M1-008).
	stored, err := settings.New(db).All(ctx)
	if err != nil {
		return resetMFAResult{}, err
	}
	policy, err := authn.DecodeMFAPolicy(stored)
	if err != nil {
		return resetMFAResult{}, err
	}
	result.MFAStillRequired = policy.Requires(user)

	totps := identity.NewTOTPs(db)
	if _, err := totps.ByUserID(ctx, user.ID); err == nil {
		result.AuthenticatorRemoved = true
	} else if !errors.Is(err, apierr.ErrNotFound) {
		return resetMFAResult{}, err
	}

	codes := identity.NewRecoveryCodes(db)
	unused, err := codes.CountUnused(ctx, user.ID)
	if err != nil {
		return resetMFAResult{}, err
	}
	result.RecoveryCodesRemoved = unused

	if err := totps.Delete(ctx, user.ID); err != nil {
		return resetMFAResult{}, err
	}
	if err := codes.DeleteForUser(ctx, user.ID); err != nil {
		return resetMFAResult{}, err
	}
	// And any sign-in that was waiting for a code. It is unanswerable now that
	// the enrolment is gone, but a challenge outliving the factor it was opened
	// against is exactly the sort of leftover this command exists to clear.
	if err := identity.NewMFAChallenges(db).DeleteForUser(ctx, user.ID); err != nil {
		return resetMFAResult{}, err
	}
	result.ChallengesDiscarded = true

	return result, nil
}

// writeResetMFA says what just happened in a way that is hard to skim past. The
// warning is the point of the command's output, and there are two of them
// because there are two outcomes: on an account nothing requires a factor of,
// the operator has just made a password sufficient where it was not; on one a
// policy still covers, they have turned a lockout into an enrolment (M1-008).
// Printing the first sentence in the second case would be telling an operator
// their deployment is less protected than it is.
func writeResetMFA(w *tabwriter.Writer, result resetMFAResult) {
	if !result.AuthenticatorRemoved && result.RecoveryCodesRemoved == 0 {
		fmt.Fprintf(w, "%s had no second factor. Nothing was removed.\n", result.Email)
		return
	}

	fmt.Fprintf(w, "Reset the second factor of %s.\n\n", result.Email)
	fmt.Fprintf(w, "id\t%s\n", result.ID)
	fmt.Fprintf(w, "authenticator\t%s\n", removedOrNot(result.AuthenticatorRemoved))
	fmt.Fprintf(w, "unused recovery codes\t%s\n", plural(result.RecoveryCodesRemoved, "code")+" removed")
	fmt.Fprintf(w, "\n")
	if result.MFAStillRequired {
		fmt.Fprintf(w, "NOTE: a second factor is still required of %s.\n", result.Email)
		fmt.Fprintf(w, "Their next sign-in can do one thing: enrol a new authenticator. The\n")
		fmt.Fprintf(w, "password alone does not get them into the application, and this command\n")
		fmt.Fprintf(w, "has not changed that — it removed the factor they could no longer use.\n")
		fmt.Fprintf(w, "They will be shown a new set of recovery codes; the old ones no longer\n")
		fmt.Fprintf(w, "work.\n")
		return
	}

	fmt.Fprintf(w, "WARNING: %s now signs in with a password and nothing else.\n", result.Email)
	fmt.Fprintf(w, "Anyone holding that password holds the account. Have them enrol an\n")
	fmt.Fprintf(w, "authenticator again as soon as they are back in, and take a new set of\n")
	fmt.Fprintf(w, "recovery codes when they do — the old ones no longer work.\n")
}

func removedOrNot(removed bool) string {
	if removed {
		return "removed"
	}
	return "none was enrolled"
}

func platformRole(admin bool) identity.PlatformRole {
	if admin {
		return identity.PlatformRoleAdmin
	}
	return identity.PlatformRoleMember
}

// createUser writes the account and the local login method that points at it.
//
// The two are separate writes because they are separate repositories, and there
// is a window between them. It is reported rather than hidden: an account
// without its identity row can still sign in — local login resolves by email —
// but M1-009's account linking reads that table, so a deployment should not be
// left with a gap in it quietly.
func createUser(ctx context.Context, db *store.DB, in identity.NewUser) (identity.User, error) {
	created, err := identity.NewUsers(db).Create(ctx, in)
	if errors.Is(err, apierr.ErrConflict) {
		// The repository's message is written for an API client, which is told a
		// code and a sentence. Here it is the whole of what an operator sees, so
		// it names the address they typed.
		return identity.User{}, fmt.Errorf(
			"%s already belongs to an account; addresses are matched without regard to case", in.Email)
	}
	if err != nil {
		return identity.User{}, err
	}

	// The subject of a local identity is the normalized address, which is what
	// the database stores as email_normalized and what every lookup uses.
	_, err = identity.NewIdentities(db).Create(ctx, identity.NewIdentity{
		UserID:   created.ID,
		Provider: identity.ProviderLocal,
		Subject:  strings.ToLower(strings.TrimSpace(created.Email)),
	})
	if err != nil {
		return identity.User{}, fmt.Errorf(
			"the account %s was created (id %s) but its local login method was not: %w",
			created.Email, created.ID, err)
	}
	return created, nil
}

// requireMigrated refuses to write to a database whose schema this binary has
// not finished applying.
//
// Without it the failure is the driver's — "Table with name user does not
// exist" — which is accurate and tells an operator nothing about what to do. The
// server migrates on startup, so this is only ever hit on a database that has
// never had one.
func (a *app) requireMigrated(ctx context.Context, db *store.DB) error {
	cfg, err := a.settings()
	if err != nil {
		return err
	}
	report, err := a.readMigrations(ctx, db, cfg)
	if err != nil {
		return err
	}
	if report.Pending > 0 {
		return fmt.Errorf("%s is at schema version %04d of %04d, with %s to apply;\n"+
			"run `blctl migrate up` first, or start the server once — it migrates on startup",
			cfg.Database.Path, report.SchemaVersion, report.ExpectedSchemaVersion,
			plural(report.Pending, "migration"))
	}
	return nil
}

// prompts are written to stderr, not stdout: they are not the command's result,
// and a caller piping stdout to a file should still see them.
const (
	passwordPrompt = "Password: "
	confirmPrompt  = "Repeat the password: "
)

// readPassword takes the password from the terminal, or from stdin when there
// is not one.
//
// On a terminal it is asked for twice and not echoed — a mistyped password on
// the bootstrap account is a locked-out deployment, and it is the one account
// nobody can reset for you. Piped in, it is read once: whatever produced it can
// be run again.
func (a *app) readPassword() (password.Plaintext, error) {
	if file, ok := a.in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		return a.promptForPassword(file)
	}

	raw, err := io.ReadAll(a.in)
	if err != nil {
		return "", fmt.Errorf("reading the password from stdin: %w", err)
	}
	// One trailing newline, because `echo secret | blctl …` adds one and
	// almost nobody remembers -n. Anything further in is the caller's, including
	// trailing spaces, which are legal in a password and not ours to remove.
	plaintext := password.Plaintext(strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r"))
	switch {
	case plaintext == "":
		return "", errors.New("no password on stdin, and stdin is not a terminal to ask on\n\n" +
			"    printf '%s' \"$PASSWORD\" | blctl user create --email a@b.c --name Alice")
	case strings.ContainsAny(plaintext.Reveal(), "\r\n"):
		return "", errors.New("the password read from stdin contains a line break; " +
			"pipe exactly the password and nothing else")
	}
	return plaintext, nil
}

// promptForPassword asks twice on the terminal, with echo off.
func (a *app) promptForPassword(tty *os.File) (password.Plaintext, error) {
	first, err := readHidden(tty, a.errOut, passwordPrompt)
	if err != nil {
		return "", err
	}
	again, err := readHidden(tty, a.errOut, confirmPrompt)
	if err != nil {
		return "", err
	}
	if first != again {
		// No hint about how they differ. This is a terminal, and the fix is to
		// type it again.
		return "", errors.New("the two passwords do not match")
	}
	return first, nil
}

// readHidden writes a prompt and reads a line with echo off, restoring the
// terminal afterwards — including when the read fails, which is what term's own
// ReadPassword guarantees.
func readHidden(tty *os.File, prompt io.Writer, message string) (password.Plaintext, error) {
	if _, err := fmt.Fprint(prompt, message); err != nil {
		return "", err
	}
	raw, err := term.ReadPassword(int(tty.Fd()))
	// The newline the user's Return did not echo, so the next prompt starts on
	// its own line.
	fmt.Fprintln(prompt)
	if err != nil {
		return "", fmt.Errorf("reading the password: %w", err)
	}
	return password.Plaintext(raw), nil
}

// policyFailure renders a rejected password as a sentence.
//
// password.Validate reports the API's field-error shape, which is right for a
// form and wrong for a terminal. The wording is the API's, so an operator and a
// user are told the same thing about the same password.
func policyFailure(err error) error {
	var apiErr *apierr.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	messages := make([]string, 0, len(apiErr.Fields()))
	for _, field := range apiErr.Fields() {
		messages = append(messages, field.Message)
	}
	if len(messages) == 0 {
		return err
	}
	return fmt.Errorf("the password %s", strings.Join(messages, "; and "))
}
