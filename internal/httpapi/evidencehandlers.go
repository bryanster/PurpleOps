package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/evidence"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// Evidence handlers (M3-009).

// UploadEvidence accepts a multipart file upload and links it to an execution.
// Side enforcement: red seat → side=red only, blue → blue, lead → either,
// admin with no seat may write either.
func (h *handlers) UploadEvidence(ctx context.Context,
	request gen.UploadEvidenceRequestObject) (gen.UploadEvidenceResponseObject, error) {

	actor, ok := authn.SubjectFrom(ctx)
	if !ok {
		return nil, apierr.Unauthenticated("no subject")
	}

	// Walk the multipart body to extract file, caption, and side.
	var file io.Reader
	var caption string
	var filename string
	var side gen.EvidenceSide

	for {
		part, err := request.Body.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, apierr.Validation(apierr.Field("body", "invalid multipart body"))
		}
		switch part.FormName() {
		case "file":
			file = part
			filename = part.FileName()
		case "caption":
			b, err := io.ReadAll(io.LimitReader(part, 4096))
			if err != nil {
				return nil, fmt.Errorf("evidence: read caption: %w", err)
			}
			caption = string(b)
		case "side":
			b, err := io.ReadAll(io.LimitReader(part, 16))
			if err != nil {
				return nil, fmt.Errorf("evidence: read side: %w", err)
			}
			side = gen.EvidenceSide(strings.TrimSpace(string(b)))
		}
	}
	if file == nil {
		return nil, apierr.Validation(apierr.Field("file", "is required"))
	}
	if !side.Valid() {
		return nil, apierr.Validation(apierr.Field("side", "must be red or blue"))
	}

	storeSide := storengagement.EvidenceSide(string(side))

	exec, err := h.engagements.GetExecution(ctx, request.ExecutionId.String())
	if err != nil {
		return nil, err
	}

	// Load parent chain for engagement check and seat lookup.
	step, err := h.engagements.GetStep(ctx, exec.StepID)
	if err != nil {
		return nil, err
	}
	scenario, err := h.engagements.GetScenario(ctx, step.ScenarioID)
	if err != nil {
		return nil, err
	}

	// Side enforcement.
	if err := enforceEvidenceSide(ctx, h.ownership, actor.PlatformRole, actor.UserID, scenario.EngagementID, storeSide); err != nil {
		return nil, err
	}

	// Closed engagement → 409.
	eng, err := h.engagements.Get(ctx, scenario.EngagementID)
	if err != nil {
		return nil, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return nil, apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	// Sniff the MIME type from the first 512 bytes (the standard magic number
	// window). Reconstruct the stream so the store receives every byte.
	sniffBuf := make([]byte, 512)
	n, sniffErr := io.ReadFull(file, sniffBuf)
	if sniffErr != nil && sniffErr != io.ErrUnexpectedEOF && sniffErr != io.EOF {
		return nil, fmt.Errorf("evidence: read sniff: %w", sniffErr)
	}
	sniffed := sniffBuf[:n]
	mime := http.DetectContentType(sniffed)
	file = io.MultiReader(bytes.NewReader(sniffed), file)

	// Validate against the configured MIME allowlist.
	if len(h.evidenceMIMEAllowlist) > 0 {
		if !mimeAllowed(mime, h.evidenceMIMEAllowlist) {
			return nil, apierr.Validation(apierr.Field("file",
				fmt.Sprintf("content type %q is not allowed; allowed types: %s",
					mime, strings.Join(h.evidenceMIMEAllowlist, ", "))))
		}
	}

	if h.evidenceStore == nil {
		return nil, apierr.Internal(fmt.Errorf("httpapi: evidence store is not configured"))
	}

	// Stream to blob store.
	sha256, _, size, err := h.evidenceStore.Put(ctx, file, mime, scenario.EngagementID)
	if err != nil {
		var tooLarge *evidence.ErrTooLarge
		var quota *evidence.ErrEngagementQuota
		switch {
		case errors.As(err, &tooLarge):
			return nil, apierr.PayloadTooLarge(fmt.Sprintf("file exceeds %d byte upload limit", tooLarge.Limit))
		case errors.As(err, &quota):
			return nil, apierr.PayloadTooLarge(fmt.Sprintf("engagement quota exceeded (%d bytes used, would add %d bytes, limit %d bytes)",
				quota.Used, quota.Got, quota.Limit))
		default:
			return nil, err
		}
	}

	// Create evidence metadata row.
	ev, err := h.evidenceRepo.Create(ctx, storengagement.NewEvidence{
		Filename:    filename,
		Caption:     caption,
		Side:        storeSide,
		ExecutionID: exec.ID,
		UploadedBy:  actor.UserID,
		Size:        size,
		MIME:        mime,
	})
	if err != nil {
		return nil, err
	}

	// Activity.
	if h.activity != nil {
		//nolint:errcheck // best-effort audit trail; failure is logged by the store
		_ = h.activity.RecordAlone(ctx, events.Entry{
			ActorID:      actor.UserID,
			Verb:         events.VerbEvidenceUploaded,
			ObjectType:   events.ObjectEvidence,
			ObjectID:     ev.ID,
			EngagementID: scenario.EngagementID,
			Delta: events.Delta(map[string]any{
				"sha256":   sha256[:12],
				"filename": ev.Filename,
				"side":     string(storeSide),
				"size":     size,
			}),
		})
	}

	wire, err := evidenceToWire(ev)
	if err != nil {
		return nil, err
	}
	return gen.UploadEvidence201JSONResponse(wire), nil
}

// ListEvidenceByExecution returns evidence linked to an execution, newest first.
func (h *handlers) ListEvidenceByExecution(ctx context.Context,
	request gen.ListEvidenceByExecutionRequestObject) (gen.ListEvidenceByExecutionResponseObject, error) {

	list, err := h.evidenceRepo.ListByExecution(ctx, request.ExecutionId.String())
	if err != nil {
		return nil, err
	}
	wire := make([]gen.Evidence, len(list))
	for i, e := range list {
		w, err := evidenceToWire(e)
		if err != nil {
			return nil, err
		}
		wire[i] = w
	}
	return gen.ListEvidenceByExecution200JSONResponse(wire), nil
}

// GetEvidence returns evidence metadata by id.
func (h *handlers) GetEvidence(ctx context.Context,
	request gen.GetEvidenceRequestObject) (gen.GetEvidenceResponseObject, error) {

	ev, err := h.evidenceRepo.ByID(ctx, request.EvidenceId.String())
	if err != nil {
		return nil, err
	}
	concealed, err := h.evidenceConcealed(ctx, ev)
	if err != nil {
		return nil, err
	}
	if concealed {
		return nil, apierr.NotFound("evidence", request.EvidenceId.String())
	}

	wire, err := evidenceToWire(ev)
	if err != nil {
		return nil, err
	}
	return gen.GetEvidence200JSONResponse(wire), nil
}

// GetEvidenceContent streams the evidence blob file to the client with safe
// download headers.
func (h *handlers) GetEvidenceContent(ctx context.Context,
	request gen.GetEvidenceContentRequestObject) (gen.GetEvidenceContentResponseObject, error) {

	ev, err := h.evidenceRepo.ByID(ctx, request.EvidenceId.String())
	if err != nil {
		return nil, err
	}
	concealed, err := h.evidenceConcealed(ctx, ev)
	if err != nil {
		return nil, err
	}
	if concealed {
		return nil, apierr.NotFound("evidence", request.EvidenceId.String())
	}

	rc, err := h.evidenceStore.Open(ev.BlobSHA256)
	if err != nil {
		return nil, err
	}

	return gen.GetEvidenceContent200ApplicationoctetStreamResponse{
		Body:          rc,
		ContentLength: ev.Size,
	}, nil
}

// DeleteEvidence removes an evidence metadata row and decrements the blob
// refcount. Uploader, lead, or admin may delete.
func (h *handlers) DeleteEvidence(ctx context.Context,
	request gen.DeleteEvidenceRequestObject) (gen.DeleteEvidenceResponseObject, error) {

	actor, ok := authn.SubjectFrom(ctx)
	if !ok {
		return nil, apierr.Unauthenticated("no subject")
	}

	ev, err := h.evidenceRepo.ByID(ctx, request.EvidenceId.String())
	if err != nil {
		return nil, err
	}

	// Check permission: uploader, lead, or admin.
	// Resolve engagement ID from the evidence's execution chain.
	var engagementID string
	if ev.ExecutionID != "" {
		exec, err := h.engagements.GetExecution(ctx, ev.ExecutionID)
		if err == nil {
			step, err := h.engagements.GetStep(ctx, exec.StepID)
			if err == nil {
				sc, err := h.engagements.GetScenario(ctx, step.ScenarioID)
				if err == nil {
					engagementID = sc.EngagementID
				}
			}
		}
	}

	if err := canDeleteEvidence(ctx, h.ownership, actor.PlatformRole, actor.UserID, engagementID, ev.UploadedBy); err != nil {
		return nil, err
	}

	// Delete the evidence link.
	if err := h.evidenceRepo.DeleteLink(ctx, ev.ID); err != nil {
		return nil, err
	}

	// Decrement blob refcount; GC the file if it reaches zero.
	gc, err := h.blobRepo.DecrementRef(ctx, ev.BlobSHA256)
	if err != nil {
		return nil, err
	}
	if gc {
		if err := h.evidenceStore.RemoveBlobFile(ev.BlobSHA256); err != nil {
			return nil, err
		}
		//nolint:errcheck // best-effort blob row cleanup after file GC
		_ = h.blobRepo.DeleteBlob(ctx, ev.BlobSHA256)
	}

	// Activity.
	if h.activity != nil {
		var engID string
		if ev.ExecutionID != "" {
			exec, err := h.engagements.GetExecution(ctx, ev.ExecutionID)
			if err == nil {
				step, err := h.engagements.GetStep(ctx, exec.StepID)
				if err == nil {
					sc, err := h.engagements.GetScenario(ctx, step.ScenarioID)
					if err == nil {
						engID = sc.EngagementID
					}
				}
			}
		}
		//nolint:errcheck // best-effort audit trail; failure is logged by the store
		_ = h.activity.RecordAlone(ctx, events.Entry{
			ActorID:      actor.UserID,
			Verb:         events.VerbEvidenceDeleted,
			ObjectType:   events.ObjectEvidence,
			ObjectID:     ev.ID,
			EngagementID: engID,
			Delta: events.Delta(map[string]any{
				"sha256": ev.BlobSHA256[:12],
			}),
		})
	}

	return gen.DeleteEvidence204Response{}, nil
}

// evidenceToWire converts a store evidence row to its OpenAPI representation.
func evidenceToWire(e storengagement.Evidence) (gen.Evidence, error) {
	execID, err := parseUUID(e.ExecutionID)
	if err != nil {
		return gen.Evidence{}, err
	}
	id, err := parseUUID(e.ID)
	if err != nil {
		return gen.Evidence{}, err
	}
	w := gen.Evidence{
		Id:          id,
		BlobSha256:  e.BlobSHA256,
		Filename:    e.Filename,
		Caption:     e.Caption,
		Side:        gen.EvidenceSide(e.Side),
		ExecutionId: execID,
		UploadedBy:  e.UploadedBy,
		UploadedAt:  e.UploadedAt,
		Size:        int(e.Size),
		Mime:        e.MIME,
	}
	if e.CommentID != "" {
		cid, err := parseUUID(e.CommentID)
		if err != nil {
			return gen.Evidence{}, err
		}
		w.CommentId.Set(cid)
	}
	return w, nil
}

// mimeAllowed reports whether mime matches an entry in the allowlist.
// Comparison is case-insensitive and strips parameters (so "image/png;
// charset=utf-8" matches "image/png").
func mimeAllowed(mime string, allowlist []string) bool {
	// Strip parameters: "text/plain; charset=utf-8" → "text/plain"
	if idx := strings.IndexByte(mime, ';'); idx != -1 {
		mime = strings.TrimSpace(mime[:idx])
	}
	mime = strings.ToLower(strings.TrimSpace(mime))
	for _, a := range allowlist {
		if strings.ToLower(strings.TrimSpace(a)) == mime {
			return true
		}
	}
	return false
}

// evidenceParent resolves an evidence row to its owning engagement id and its
// parent step id, walking execution (or comment -> execution) -> step ->
// scenario. A missing row anywhere in the chain is the same not-found the
// middleware produces, so a child id from another engagement is concealed.
func (h *handlers) evidenceParent(ctx context.Context, ev storengagement.Evidence) (engagementID, stepID string, err error) {
	var exec storengagement.Execution
	if ev.ExecutionID != "" {
		exec, err = h.engagements.GetExecution(ctx, ev.ExecutionID)
	} else {
		var c storengagement.Comment
		c, err = h.engagements.GetComment(ctx, ev.CommentID)
		if err != nil {
			return "", "", err
		}
		exec, err = h.engagements.GetExecution(ctx, c.ExecutionID)
	}
	if err != nil {
		return "", "", err
	}
	engagementID, err = h.engagements.StepEngagementID(ctx, exec.StepID)
	if err != nil {
		return "", "", err
	}
	return engagementID, exec.StepID, nil
}

// evidenceConcealed reports whether a blue member of a blind engagement must be
// denied this evidence because its parent step is unrevealed. It is the
// handler-side mirror of the middleware's blind guard, kept here because
// middleware mappings drift (M7-012).
func (h *handlers) evidenceConcealed(ctx context.Context, ev storengagement.Evidence) (bool, error) {
	engagementID, stepID, err := h.evidenceParent(ctx, ev)
	if err != nil {
		return false, err
	}
	step, err := h.engagements.GetStep(ctx, stepID)
	if err != nil {
		return false, err
	}
	scope, err := h.stepBlindScope(ctx, engagementID)
	if err != nil {
		return false, err
	}
	return !scope.Permits(step.RevealedAt != nil), nil
}
