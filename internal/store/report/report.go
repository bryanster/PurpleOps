package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/bryanster/blacklight/internal/store"
	"time"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// DB is the part of the store these repositories need.
type DB interface {
	Read() store.Reader
	Write(ctx context.Context, fn func(tx *sql.Tx) error) error
}
type After func(ctx context.Context, tx *sql.Tx) error

// Report is one draft report, owned by an engagement.
type Report struct {
	ID           string
	EngagementID string
	Title        string

	// Branding overrides. Nil = fall back to install defaults.
	ClientName  *string
	LogoBlobRef *string
	Colours     json.RawMessage // nullable JSON object

	CreatedBy string
	CreatedAt time.Time
	UpdatedBy *string
	UpdatedAt time.Time
}

// ReportBlock is one ordered block instance in a draft.
type ReportBlock struct {
	ID       string
	ReportID string
	Ordinal  int
	BlockID  string
	Params   json.RawMessage
}

// NewReport describes the caller's half of creating a report.
type NewReport struct {
	EngagementID string
	Title        string
	CreatedBy    string
}

// ReportUpdate describes the caller's half of patching a report.
type ReportUpdate struct {
	Title       *string
	ClientName  **string // double pointer: nil = no change, *nil = clear
	LogoBlobRef **string
	Colours     *json.RawMessage // nil = no change, non-nil JSON = set/clear
	UpdatedBy   string
}

// newID returns a UUIDv7 string. Tests can shadow it.
var newID = func() string { return uuid.Must(uuid.NewV7()).String() }

// now returns the current time in UTC. Tests can shadow it.
var now = func() time.Time { return time.Now().UTC() }

func requireOneRow(result sql.Result, resource, id string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return apierr.NotFound(resource, id)
	}
	return nil
}
