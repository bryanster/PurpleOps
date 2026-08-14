// Package bootstrap creates the first administrator of a deployment from the
// environment, so that a deployment tool can hand over something signable-in.
//
// It exists for one shape of deployment. There is no sign-up, so the first
// account is made with `blctl user create` — which needs the database file, and
// DuckDB gives that file to one process at a time. Where the database lives
// inside a managed container (Azure Container Apps, Cloud Run, ECS), running
// the CLI means stopping the server, mounting the same volume somewhere else,
// running one command and starting the server again. Terraform can provision
// everything about such a deployment except the one account needed to open it,
// which is a poor place to leave an operator.
//
// The rule that makes this safe to leave configured for the life of a
// deployment is [Apply]'s precondition: it does nothing unless the database
// holds no accounts at all. Not "no account with this address" — no accounts.
// So it runs at most once per database, and afterwards the configuration is
// inert: it cannot reset a password somebody changed, cannot re-promote an
// account that was demoted, and cannot bring back one that was disabled. An
// environment variable that could do any of those would be a way in for
// everything that can read a container's environment.
//
// Everywhere else, keep using the CLI. A password that goes through an
// orchestrator is a password that orchestrator has, and `blctl user create`
// takes one on a terminal and nowhere else.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Apply creates the account described by cfg, if there is one to create.
//
// It is a no-op, and no error, in the two cases that are not mistakes: no
// address configured, and a database that already has accounts. Everything
// after that point is a mistake worth failing the start for — an unreadable
// secret file, a password the policy refuses — because the alternative is a
// deployment that comes up healthy and cannot be signed in to, with the reason
// a line in a log nobody is reading yet.
//
// It runs after migrations, on a server that is not yet listening.
func Apply(ctx context.Context, db *store.DB, cfg config.Bootstrap, log *slog.Logger) error {
	if !cfg.Enabled() {
		return nil
	}

	count, err := identity.NewUsers(db).Count(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	if count > 0 {
		// Info rather than silence: an operator who expected an account and did
		// not get one is owed the reason, and the reason is almost always that
		// this is not a fresh database.
		log.InfoContext(ctx, "first administrator not created: this deployment already has accounts",
			slog.String("email", cfg.Email),
			slog.Int("accounts", count))
		return nil
	}

	plaintext, err := readPassword(cfg)
	if err != nil {
		return err
	}
	if err := password.Validate("password", plaintext); err != nil {
		return fmt.Errorf("bootstrap: the first administrator's password %s (%s or %s)",
			policyFailure(err), config.EnvBootstrapPassword, config.EnvBootstrapPasswordFile)
	}
	hash, err := password.Hash(plaintext)
	if err != nil {
		return fmt.Errorf("bootstrap: hash the first administrator's password: %w", err)
	}

	created, err := identity.CreateWithLocalLogin(ctx, db, identity.NewUser{
		Email:        strings.TrimSpace(cfg.Email),
		DisplayName:  strings.TrimSpace(cfg.Name),
		PasswordHash: hash,
		PlatformRole: authz.PlatformRoleAdmin,
		Status:       identity.StatusActive,
	})
	if err != nil {
		// The conflict cannot happen — nothing else has written to a database
		// with no accounts in it — but if it somehow did, an operator should be
		// told which address rather than shown a bare code.
		if errors.Is(err, apierr.ErrConflict) {
			return fmt.Errorf("bootstrap: %s already belongs to an account", cfg.Email)
		}
		return fmt.Errorf("bootstrap: create the first administrator: %w", err)
	}

	// Warn, not info. This line says a password that was written in a
	// deployment tool now opens an administrator account, and it is the only
	// notice anybody gets that it should be changed.
	log.WarnContext(ctx, "created the first administrator from the environment; "+
		"sign in and change the password, then remove the bootstrap configuration",
		slog.String("user_id", created.ID),
		slog.String("email", created.Email))
	return nil
}

// readPassword takes the initial password from the file if there is one, and
// from the variable otherwise. [config.Config] has already refused the
// configurations where both or neither is set.
//
// The file is read here rather than at startup so that a deployment which has
// long since created its administrator is not stopped from starting by a secret
// it no longer needs being unmounted.
func readPassword(cfg config.Bootstrap) (password.Plaintext, error) {
	if cfg.PasswordFile == "" {
		return password.Plaintext(string(cfg.Password.Reveal())), nil
	}

	raw, err := os.ReadFile(cfg.PasswordFile)
	if err != nil {
		return "", fmt.Errorf("bootstrap: reading the first administrator's password from %s: %w",
			config.EnvBootstrapPasswordFile, err)
	}
	// One trailing newline, because almost everything that writes a file writes
	// one. Anything further in is the password's, including trailing spaces,
	// which are legal and not ours to remove — the same rule `blctl user
	// create` applies to a password on stdin.
	plaintext := password.Plaintext(strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r"))
	if strings.ContainsAny(plaintext.Reveal(), "\r\n") {
		return "", fmt.Errorf("bootstrap: the password in %s (%s) contains a line break; "+
			"the file must hold exactly the password and nothing else",
			cfg.PasswordFile, config.EnvBootstrapPasswordFile)
	}
	return plaintext, nil
}

// policyFailure renders a rejected password as the fragment of a sentence, the
// way the CLI does: the wording is the API's, so an operator and a person
// choosing their own password are told the same thing about the same password.
func policyFailure(err error) string {
	var apiErr *apierr.Error
	if !errors.As(err, &apiErr) {
		return "was refused: " + err.Error()
	}
	messages := make([]string, 0, len(apiErr.Fields()))
	for _, field := range apiErr.Fields() {
		messages = append(messages, field.Message)
	}
	if len(messages) == 0 {
		return "was refused: " + err.Error()
	}
	return strings.Join(messages, "; and ")
}
