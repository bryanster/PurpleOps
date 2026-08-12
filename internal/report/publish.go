package report

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store/blind"
	storereport "github.com/bryanster/blacklight/internal/store/report"
)

// PublishService creates immutable published versions of report drafts (M6-011).
// Construct with [NewPublishService].
type PublishService struct {
	reports  *storereport.Reports
	versions *storereport.Versions
	renderer *DocumentRenderer
	resolver *BrandingResolver
	activity *events.Log
}

// PublishDeps is everything a PublishService needs.
type PublishDeps struct {
	Reports  *storereport.Reports
	Versions *storereport.Versions
	Renderer *DocumentRenderer
	Resolver *BrandingResolver
	Activity *events.Log // optional; nil skips durable activity rows
}

// NewPublishService returns a PublishService over deps, or an error naming
// what is missing.
func NewPublishService(deps PublishDeps) (*PublishService, error) {
	var missing []string
	if deps.Reports == nil {
		missing = append(missing, "Reports")
	}
	if deps.Versions == nil {
		missing = append(missing, "Versions")
	}
	if deps.Renderer == nil {
		missing = append(missing, "Renderer")
	}
	if deps.Resolver == nil {
		missing = append(missing, "Resolver")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("report: publish: missing dependencies: %s", joinStrings(missing))
	}
	return &PublishService{
		reports:  deps.Reports,
		versions: deps.Versions,
		renderer: deps.Renderer,
		resolver: deps.Resolver,
		activity: deps.Activity,
	}, nil
}

// PublishInput is the caller's half of publishing a report draft.
type PublishInput struct {
	ReportID        string
	EngagementID    string
	EngagementName  string
	PublishedBy     string
	IncludeEvidence bool
}

// PublishResult is what Publish returns on success.
type PublishResult struct {
	Version storereport.ReportVersion
}

// Publish freezes the current draft as an immutable published version.
// It renders with lead/full blind scope always, regardless of caller's seat.
// The publish fails entirely if any block errors during render.
func (s *PublishService) Publish(ctx context.Context, env RenderEnv, in PublishInput) (PublishResult, error) {
	// 1. Load the report and its blocks.
	rep, err := s.reports.ByID(ctx, in.ReportID)
	if err != nil {
		return PublishResult{}, fmt.Errorf("report: publish: %w", err)
	}

	blocks, err := s.reports.BlocksByReport(ctx, in.ReportID)
	if err != nil {
		return PublishResult{}, fmt.Errorf("report: publish: blocks: %w", err)
	}

	// 2. Resolve branding at publish time.
	branding, err := s.resolver.Resolve(ctx, rep)
	if err != nil {
		return PublishResult{}, fmt.Errorf("report: publish: branding: %w", err)
	}

	// 3. Render with lead/full scope always.
	publishEnv := env
	publishEnv.Branding = branding
	publishEnv.IncludeEvidence = in.IncludeEvidence
	publishEnv.BlindScope = blind.Scope{} // lead/full: not blind

	doc := s.renderer.RenderDocument(ctx, rep, blocks, publishEnv)

	// 4. Fail publish if any block errored.
	if len(doc.Warnings) > 0 {
		return PublishResult{}, fmt.Errorf("report: publish: %d block(s) errored during render: %v",
			len(doc.Warnings), doc.Warnings)
	}

	// 5. Compute content hash.
	contentHash := storereport.HashBytes(doc.HTML)

	// 6. Serialize blocks and branding to JSON for freezing.
	blocksJSON, err := json.Marshal(blocks)
	if err != nil {
		return PublishResult{}, fmt.Errorf("report: publish: marshal blocks: %w", err)
	}
	brandingJSON, err := json.Marshal(branding)
	if err != nil {
		return PublishResult{}, fmt.Errorf("report: publish: marshal branding: %w", err)
	}

	// 7. Determine the next ordinal.
	ordinal, err := s.versions.NextOrdinal(ctx, in.ReportID)
	if err != nil {
		return PublishResult{}, fmt.Errorf("report: publish: ordinal: %w", err)
	}

	// 8. Insert the immutable version row.
	version, err := s.versions.Insert(ctx, storereport.NewVersion{
		ReportID:        in.ReportID,
		Ordinal:         ordinal,
		Title:           rep.Title,
		PublishedBy:     in.PublishedBy,
		IncludeEvidence: in.IncludeEvidence,
		BlindScope:      "lead_full",
		BlocksJSON:      string(blocksJSON),
		BrandingJSON:    string(brandingJSON),
		HTML:            string(doc.HTML),
		ContentSHA256:   contentHash,
	})
	if err != nil {
		return PublishResult{}, fmt.Errorf("report: publish: insert version: %w", err)
	}

	// 9. Record activity (separate tx, consistent with report CRUD pattern).
	if s.activity != nil {
		if err := s.activity.RecordAlone(ctx, events.Entry{
			EngagementID: in.EngagementID,
			ActorID:      in.PublishedBy,
			Verb:         events.VerbReportPublished,
			ObjectType:   events.ObjectReportVersion,
			ObjectID:     version.ID,
			Delta:        map[string]any{"ordinal": ordinal},
		}); err != nil {
			return PublishResult{}, fmt.Errorf("report: publish: activity: %w", err)
		}
	}

	return PublishResult{Version: version}, nil
}

// joinStrings is a simple strings.Join helper to avoid importing strings
// just for a single use.
func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	s := ss[0]
	for _, next := range ss[1:] {
		s += ", " + next
	}
	return s
}
