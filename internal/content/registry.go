package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// Registry is the source-registry surface (M2-002): list, detail, enable,
// disable, patch metadata, delete, and list versions. Construct with [New].
//
// Authorization is not decided here. Callers that reached these methods already
// hold content.read or content.manage via the HTTP middleware (or are blctl,
// which is the host's access control).
type Registry struct {
	sources  *storecontent.Sources
	versions *storecontent.Versions
	jobs     *storecontent.Jobs
	activity *events.Log
	policy   URLPolicy
}

// Deps is everything a [Registry] is built from.
type Deps struct {
	Sources  *storecontent.Sources
	Versions *storecontent.Versions
	Jobs     *storecontent.Jobs
	Activity *events.Log // optional; nil skips durable activity rows
	// Policy fences source URLs on write (M7-014). The zero value is the
	// production posture: https only, no private destinations.
	Policy URLPolicy
}

// New returns a Registry over deps, or an error naming what is missing.
func New(deps Deps) (*Registry, error) {
	switch {
	case deps.Sources == nil:
		return nil, errors.New("content: no source repository")
	case deps.Versions == nil:
		return nil, errors.New("content: no version repository")
	case deps.Jobs == nil:
		return nil, errors.New("content: no job repository")
	}
	return &Registry{
		sources:  deps.Sources,
		versions: deps.Versions,
		jobs:     deps.Jobs,
		activity: deps.Activity,
		policy:   deps.Policy,
	}, nil
}

// SourceFilter is the list filter the HTTP layer may pass. Kind is the wire
// spelling; empty means no filter. Enabled is a pointer so false is distinct
// from "not mentioned".
type SourceFilter struct {
	Kind    string
	Enabled *bool
}

// SourceDetail is a registry row plus the most recent job, when any exists.
// List endpoints return the row alone; detail includes the summary so an admin
// UI can show "last sync failed" without a second round trip.
type SourceDetail struct {
	Source  storecontent.Source
	LastJob *storecontent.Job
}

// ListSources returns every source matching f, kind then id.
func (r *Registry) ListSources(ctx context.Context, f SourceFilter) ([]storecontent.Source, error) {
	filter, err := toStoreFilter(f)
	if err != nil {
		return nil, err
	}
	return r.sources.List(ctx, filter)
}

// GetSource returns one source and its most recent job summary, or
// [apierr.NotFound].
func (r *Registry) GetSource(ctx context.Context, id string) (SourceDetail, error) {
	src, err := r.sources.ByID(ctx, id)
	if err != nil {
		return SourceDetail{}, err
	}
	detail := SourceDetail{Source: src}

	jobs, err := r.jobs.List(ctx, storecontent.ListFilter{SourceID: id, Limit: 1})
	if err != nil {
		return SourceDetail{}, err
	}
	if len(jobs) > 0 {
		j := jobs[0]
		detail.LastJob = &j
	}
	return detail, nil
}

// SourceEdit is a patch: a nil field is one the request did not mention.
// Kind is never editable.
type SourceEdit struct {
	Name *string
	URL  *string
	Ref  *string
}

// UpdateSource applies a metadata patch and returns the source as stored.
func (r *Registry) UpdateSource(ctx context.Context, actor authn.Subject, id string, edit SourceEdit) (storecontent.Source, error) {
	before, err := r.sources.ByID(ctx, id)
	if err != nil {
		return storecontent.Source{}, err
	}

	after := before
	delta := map[string]any{}
	if edit.Name != nil {
		name := strings.TrimSpace(*edit.Name)
		if name == "" {
			return storecontent.Source{}, apierr.Validation(apierr.Field(
				"name", "name must not be empty",
			))
		}
		if name != before.Name {
			delta["name"] = change(before.Name, name)
			after.Name = name
		}
	}
	if edit.URL != nil && *edit.URL != before.URL {
		if err := r.policy.Validate(ctx, *edit.URL); err != nil {
			return storecontent.Source{}, apierr.Validation(apierr.Field("url", err.Error()))
		}
		delta["url"] = change(before.URL, *edit.URL)
		after.URL = *edit.URL
	}
	if edit.Ref != nil && *edit.Ref != before.Ref {
		delta["ref"] = change(before.Ref, *edit.Ref)
		after.Ref = *edit.Ref
	}
	if len(delta) == 0 {
		return before, nil
	}

	return r.sources.UpdateMeta(ctx, after, r.recordInTx(actor.UserID, events.VerbContentSourceUpdated, delta))
}

// EnableSource sets enabled=true. Idempotent.
func (r *Registry) EnableSource(ctx context.Context, actor authn.Subject, id string) (storecontent.Source, error) {
	return r.setEnabled(ctx, actor, id, true, events.VerbContentSourceEnabled)
}

// DisableSource sets enabled=false. Idempotent.
//
// Existing engagement data is not modified (there is none yet in M2). New
// references are refused by [AssertReferencable] once this returns.
func (r *Registry) DisableSource(ctx context.Context, actor authn.Subject, id string) (storecontent.Source, error) {
	return r.setEnabled(ctx, actor, id, false, events.VerbContentSourceDisabled)
}

func (r *Registry) setEnabled(ctx context.Context, actor authn.Subject, id string, enabled bool, verb events.Verb) (storecontent.Source, error) {
	before, err := r.sources.ByID(ctx, id)
	if err != nil {
		return storecontent.Source{}, err
	}
	if before.Enabled == enabled {
		return before, nil
	}
	delta := map[string]any{"enabled": change(before.Enabled, enabled)}
	return r.sources.SetEnabled(ctx, id, enabled, r.recordInTx(actor.UserID, verb, delta))
}

// DeleteSource hard-deletes a source and its entire content subtree in one
// write transaction.
//
// The custom seed cannot be deleted: user-authored rows need a home, and
// re-creating it is not automatic. Builtin upstream seeds may be deleted;
// disabling is the normal path, and re-seed is a fresh migrate on an empty
// database (or a future admin action), not something this call restores.
//
// External references outside content do not exist in M2, so there is nothing
// to count yet. When M3 adds engagement refs, this method must refuse with 409
// and counts before cascading.
func (r *Registry) DeleteSource(ctx context.Context, actor authn.Subject, id string) error {
	before, err := r.sources.ByID(ctx, id)
	if err != nil {
		return err
	}
	if before.Kind == storecontent.KindCustom || before.ID == storecontent.SourceIDCustom {
		return apierr.Conflict("the custom content source cannot be deleted; it is the home for user-authored rows")
	}

	delta := map[string]any{
		"kind": string(before.Kind),
		"name": before.Name,
	}
	return r.sources.DeleteCascade(ctx, id, r.recordInTx(actor.UserID, events.VerbContentSourceDeleted, delta))
}

// ListVersions returns every version snapshot under a source, or
// [apierr.NotFound] when the source itself is missing.
func (r *Registry) ListVersions(ctx context.Context, sourceID string) ([]storecontent.SourceVersion, error) {
	if _, err := r.sources.ByID(ctx, sourceID); err != nil {
		return nil, err
	}
	return r.versions.ListBySource(ctx, sourceID)
}

func (r *Registry) recordInTx(actorID string, verb events.Verb, delta map[string]any) storecontent.After {
	if r.activity == nil {
		return nil
	}
	return func(ctx context.Context, tx *sql.Tx) error {
		return r.activity.Record(ctx, tx, events.Entry{
			ActorID:    actorID,
			Verb:       verb,
			ObjectType: events.ObjectContentSource,
			ObjectID:   storecontent.AfterEntityID(ctx),
			Delta:      events.Delta(delta),
		})
	}
}

func change(from, to any) map[string]any {
	return map[string]any{"from": from, "to": to}
}

func toStoreFilter(f SourceFilter) (storecontent.SourceFilter, error) {
	out := storecontent.SourceFilter{Enabled: f.Enabled}
	if f.Kind == "" {
		return out, nil
	}
	kind, err := parseKind(f.Kind)
	if err != nil {
		return storecontent.SourceFilter{}, err
	}
	out.Kind = kind
	return out, nil
}

// parseKind resolves a wire spelling. The OpenAPI enum is the first fence; this
// is the second for blctl and any future caller that does not come through the
// request validator.
func parseKind(name string) (storecontent.Kind, error) {
	switch k := storecontent.Kind(name); k {
	case storecontent.KindAttack, storecontent.KindAtomic, storecontent.KindSigma,
		storecontent.KindCTID, storecontent.KindCustom:
		return k, nil
	default:
		return "", apierr.Validation(apierr.Field(
			"kind", fmt.Sprintf("%q is not a known content source kind", name),
		))
	}
}
