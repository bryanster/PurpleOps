// Package setup is the first-run state of an installation: whether anybody has
// yet been through the setup wizard, and the one write that says they have.
//
// # Why a state at all
//
// A fresh installation boots with an empty content library. Nothing is fetched
// at first boot — deliberately, so that the container starts in seconds and on
// a machine with no route to the internet — which means the first administrator
// signs in to a product that cannot yet map a step to a technique. Every screen
// works and none of them are useful. The wizard exists to close that gap while
// the person who can close it is looking at it, and this package is the fact it
// reads: has this installation been set up, or is it still the empty one the
// image ships.
//
// # What "completed" means, and what it does not
//
// It means somebody with `settings.manage` reached the end of the wizard. It
// does *not* mean ATT&CK is installed: the wizard can be finished by choosing
// to install nothing, which is the honest answer for an air-gapped deployment
// that will import an offline bundle later (docs/content-bundles.md). Tying the
// flag to installed content would give that deployment a wizard it can never
// dismiss, and a screen you cannot get rid of is a screen people learn to click
// through.
//
// So the flag records a decision, not an outcome. What is installed is a
// question for the content catalog, and the wizard asks it there.
//
// # Absence is the default
//
// One key, `setup.completed_at`, absent until it is written. An installation
// nobody has configured therefore needs setup, which is the answer that makes a
// fresh boot lead somewhere; and `internal/store/settings` reads an absent row
// as a zero rather than an error, so the state of a database that predates this
// package is "not set up" rather than a failure to start.
//
// Completing is idempotent and one-way. A second call keeps the first
// timestamp — it is when the installation was set up, and the wizard's Finish
// button being clicked twice is not new information — and nothing here clears
// it. An operator who wants the wizard back has `blctl setup reset`.
package setup

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store/settings"
)

// KeyCompletedAt is the platform setting this package owns: the RFC 3339
// instant the wizard was finished. It is exported because blctl reads and
// clears the same row, and two spellings of a key are two rows.
const KeyCompletedAt = "setup.completed_at"

// State is what an installation answers about its own first run.
type State struct {
	// Completed is whether the wizard has been finished. It is the only field
	// a caller has to look at; the rest is provenance.
	Completed bool

	// CompletedAt is when, in UTC, or the zero time when Completed is false.
	CompletedAt time.Time

	// CompletedBy is the user ID that finished it, or "" when the row was
	// written by something that is not a person — `blctl setup complete` on a
	// provisioning run, most likely. "Who decided this installation was
	// configured" is a question worth being able to answer.
	CompletedBy string
}

// Settings is the part of the settings store this package needs.
// [*settings.Store] satisfies it.
//
// Declared here rather than taken as a concrete type for the reason
// internal/authn declares its own: the dependency is these two calls, and a
// test substitutes them without a database.
type Settings interface {
	All(ctx context.Context) (map[string]settings.Setting, error)
	Put(ctx context.Context, values map[string]string, updatedBy string) error
}

// Service reads and writes the first-run state. Construct it with [New].
//
// Authorization is not decided here. `settings.read` and `settings.manage` are
// declared on the operations in api/openapi.yaml and enforced by the middleware
// before a handler that calls this is entered (M1-013); a second check here
// would be a second definition of who an administrator is.
type Service struct {
	settings Settings

	// activity is the durable log. Optional: nil skips the row, which is what
	// blctl does — it has no request, no session and nobody to attribute.
	activity *events.Log
}

// Deps is everything a [Service] is built from.
type Deps struct {
	Settings Settings
	Activity *events.Log // optional
}

// New returns a Service over deps, or an error naming what is missing.
func New(deps Deps) (*Service, error) {
	if deps.Settings == nil {
		return nil, errors.New("setup: no settings store")
	}
	return &Service{settings: deps.Settings, activity: deps.Activity}, nil
}

// State reports whether this installation has been through the wizard.
func (s *Service) State(ctx context.Context) (State, error) {
	stored, err := s.settings.All(ctx)
	if err != nil {
		return State{}, err
	}
	return Decode(stored)
}

// Complete marks the installation as set up and returns the state as stored.
//
// Calling it when setup is already complete is not an error and does not move
// the timestamp: the wizard is a thing that happened once, and two people
// finishing it in two browsers should agree about when. The second caller gets
// the first caller's answer, which is also what makes the endpoint safe to
// retry after a dropped response.
func (s *Service) Complete(ctx context.Context, subject authn.Subject) (State, error) {
	current, err := s.State(ctx)
	if err != nil {
		return State{}, err
	}
	if current.Completed {
		return current, nil
	}

	at := time.Now().UTC().Truncate(time.Microsecond)
	if err := s.settings.Put(ctx, map[string]string{
		KeyCompletedAt: at.Format(time.RFC3339Nano),
	}, subject.UserID); err != nil {
		return State{}, err
	}

	if s.activity != nil {
		if err := s.activity.RecordAlone(ctx, events.Entry{
			ActorID:    subject.UserID,
			Verb:       events.VerbSetupCompleted,
			ObjectType: "platform",
			ObjectID:   "setup",
			At:         at,
		}); err != nil {
			return State{}, err
		}
	}

	return State{Completed: true, CompletedAt: at, CompletedBy: subject.UserID}, nil
}

// Decode reads the first-run state out of stored settings.
//
// Exported for blctl, which has the database but no [Service] and still has to
// print the same answer the server would. One decoder, so the command line and
// the API cannot disagree about what the row means.
//
// A row that is present and unparseable is an error rather than a false. The
// only writer is [Service.Complete], so an unreadable value means the table was
// edited by hand, and answering "not set up" would put an installation that has
// been running for a year back at the wizard.
func Decode(stored map[string]settings.Setting) (State, error) {
	row, ok := stored[KeyCompletedAt]
	if !ok {
		return State{}, nil
	}
	at, err := time.Parse(time.RFC3339Nano, row.Value)
	if err != nil {
		return State{}, fmt.Errorf(
			"setup: the platform setting %q holds %q, which is not an RFC 3339 timestamp: %w",
			KeyCompletedAt, row.Value, err)
	}
	return State{
		Completed:   true,
		CompletedAt: at.UTC(),
		CompletedBy: row.UpdatedBy,
	}, nil
}
