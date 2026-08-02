package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bryanster/purpleops/internal/authn/password"
	"github.com/bryanster/purpleops/internal/httpapi/apierr"
	"github.com/bryanster/purpleops/internal/store"
	"github.com/bryanster/purpleops/internal/store/identity"
)

// `popsctl user create` is how the first administrator of a deployment gets in:
// there is no sign-up, and the web interface has nothing to offer somebody who
// cannot sign in. It is also what the end-to-end suite seeds its accounts with.

func newUserCommand(a *app) *cobra.Command {
	return group("user", "Manage user accounts", newUserCreateCommand(a))
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
			"    printf '%s' \"$PASSWORD\" | popsctl user create --email a@b.c --name Alice --admin\n\n" +
			"It is never taken from a flag or an environment variable: both end up in shell\n" +
			"history, in `ps`, and in whatever collects the logs.\n\n" +
			"The account is created active, with a local password login. The database must\n" +
			"already be migrated — run `popsctl migrate up` first, or start the server once.",
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
			"run `popsctl migrate up` first, or start the server once — it migrates on startup",
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
	// One trailing newline, because `echo secret | popsctl …` adds one and
	// almost nobody remembers -n. Anything further in is the caller's, including
	// trailing spaces, which are legal in a password and not ours to remove.
	plaintext := password.Plaintext(strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r"))
	switch {
	case plaintext == "":
		return "", errors.New("no password on stdin, and stdin is not a terminal to ask on\n\n" +
			"    printf '%s' \"$PASSWORD\" | popsctl user create --email a@b.c --name Alice")
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
