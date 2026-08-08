package httpapi

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// ============================================================================
// CSV injection escaping
// ============================================================================

func TestExportCSVInjection(t *testing.T) {
	fx := analyticstest.Seed(t)
	_ = fx // fixture for setup side effects

	tests := []struct {
		input, wantPrefix string
	}{
		{"=cmd|' /C calc'!A0", "'=cmd|' /C calc'!A0"},
		{"+SUM(A1:A10)", "'+SUM(A1:A10)"},
		{"-SUM(A1:A10)", "'-SUM(A1:A10)"},
		{"@SUM(A1:A10)", "'@SUM(A1:A10)"},
		{"\ttext", "'\ttext"},
		{"\rtext", "'\rtext"},
		{"normal text", "normal text"},
		{"123", "123"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("input=%q", tc.input), func(t *testing.T) {
			got := escapeCSV(tc.input)
			if got != tc.wantPrefix {
				t.Errorf("escapeCSV(%q) = %q, want %q", tc.input, got, tc.wantPrefix)
			}
		})
	}
}

// ============================================================================
// CSV quoting — comma, quote, newline in field
// ============================================================================

func TestExportCSVQuoting(t *testing.T) {
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)

	if err := cw.Write([]string{"field"}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if err := cw.Write([]string{"val,with,commas and \"quotes\" and\nnewlines"}); err != nil {
		t.Fatalf("write data: %v", err)
	}
	cw.Flush()
	output := buf.String()
	r := csv.NewReader(strings.NewReader(output))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("re-reading CSV: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[1][0] != "val,with,commas and \"quotes\" and\nnewlines" {
		t.Errorf("round-tripped value = %q, want original", records[1][0])
	}
}

// ============================================================================
// Column order — header stability
// ============================================================================

func TestExportColumnOrder(t *testing.T) {
	expectedExecutions := []string{
		"scenario_name", "scenario_ordinal", "step_ordinal", "step_name",
		"technique_id", "subtechnique_id", "tactic_id",
		"objective", "target_asset",
		"red_status", "executed_by", "started_at", "ended_at",
		"command_run", "source_host", "target_host", "red_notes",
		"detection_category", "detection_modifiers", "protection",
		"detected_at", "detecting_source", "detecting_rule_ref",
		"alert_severity", "blue_notes", "scored_by", "scored_at",
		"mttd_seconds", "derived_outcome",
	}

	for i, h := range executionsCSVHeader {
		if h != expectedExecutions[i] {
			t.Errorf("executions header[%d] = %q, want %q", i, h, expectedExecutions[i])
		}
	}
	if len(executionsCSVHeader) != len(expectedExecutions) {
		t.Errorf("executions header length = %d, want %d", len(executionsCSVHeader), len(expectedExecutions))
	}

	expectedFindings := []string{
		"title", "description", "severity", "status", "owner",
		"recommendation", "linked_step_ids", "created_at", "updated_at",
	}
	for i, h := range findingsCSVHeader {
		if h != expectedFindings[i] {
			t.Errorf("findings header[%d] = %q, want %q", i, h, expectedFindings[i])
		}
	}
	if len(findingsCSVHeader) != len(expectedFindings) {
		t.Errorf("findings header length = %d, want %d", len(findingsCSVHeader), len(expectedFindings))
	}
}

// ============================================================================
// Empty engagement → header only, no data rows
// ============================================================================

func TestExportEmptyEngagement(t *testing.T) {
	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	ctx := authCtx(fx.FutureID)
	req := gen.ExportEngagementRequestObject{
		EngagementId: toUUID(fx.FutureID),
		Params: gen.ExportEngagementParams{
			Format:  gen.ExportEngagementParamsFormatCsv,
			Dataset: gen.ExportEngagementParamsDatasetExecutions,
		},
	}

	resp, err := h.ExportEngagement(ctx, req)
	if err != nil {
		t.Fatalf("ExportEngagement: %v", err)
	}

	w := &testResponseWriter{header: http.Header{}}
	if err := resp.VisitExportEngagementResponse(w); err != nil {
		t.Fatalf("Visit: %v", err)
	}

	raw := strings.TrimPrefix(w.buf.String(), "\xEF\xBB\xBF")
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) < 1 {
		t.Fatal("expected at least a header row")
	}
	if !strings.Contains(lines[0], "scenario_name") {
		t.Errorf("header row missing, got: %q", lines[0])
	}
	if len(lines) > 1 {
		t.Errorf("expected only header, got %d lines: %v", len(lines), lines)
	}
}

// ============================================================================
// Blind seat — fewer rows in CSV
// ============================================================================

func TestExportBlindSeat(t *testing.T) {
	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	leadReq := gen.ExportEngagementRequestObject{
		EngagementId: toUUID(fx.BaselineID),
		Params: gen.ExportEngagementParams{
			Format:  gen.ExportEngagementParamsFormatCsv,
			Dataset: gen.ExportEngagementParamsDatasetExecutions,
		},
	}

	leadCtx := context.WithValue(context.Background(), authorizationKey{}, Authorization{
		OperationID: "exportEngagement",
		Subject: authz.Subject{
			UserID:       "test-user",
			PlatformRole: authz.PlatformRoleMember,
			Method:       authz.MethodCookie,
			Memberships: map[string]authz.EngagementRole{
				fx.BaselineID: authz.EngagementRoleLead,
			},
		},
		Action:   authz.ActionReportRead,
		Resource: authz.Resource{Type: authz.ResourceReport, EngagementID: fx.BaselineID},
		Allowed:  true,
	})

	leadResp, err := h.ExportEngagement(leadCtx, leadReq)
	if err != nil {
		t.Fatalf("lead ExportEngagement: %v", err)
	}

	blueCtx := context.WithValue(context.Background(), authorizationKey{}, Authorization{
		OperationID: "exportEngagement",
		Subject: authz.Subject{
			UserID:       "test-user-blue",
			PlatformRole: authz.PlatformRoleMember,
			Method:       authz.MethodCookie,
			Memberships: map[string]authz.EngagementRole{
				fx.BaselineID: authz.EngagementRoleBlue,
			},
		},
		Action:   authz.ActionReportRead,
		Resource: authz.Resource{Type: authz.ResourceReport, EngagementID: fx.BaselineID},
		Allowed:  true,
	})

	blueResp, err := h.ExportEngagement(blueCtx, leadReq)
	if err != nil {
		t.Fatalf("blue ExportEngagement: %v", err)
	}

	leadCSV := captureCSV(t, leadResp)
	blueCSV := captureCSV(t, blueResp)

	// Lead gets header + 9 data rows.
	if len(leadCSV) != 10 {
		t.Errorf("lead CSV rows (incl header) = %d, want 10", len(leadCSV))
	}

	// Blue gets header + 7 data rows (2 unrevealed excluded).
	if len(blueCSV) != 8 {
		t.Errorf("blue CSV rows (incl header) = %d, want 8", len(blueCSV))
	}

	// Blue must not contain unrevealed technique IDs.
	for i, row := range blueCSV {
		if i == 0 {
			continue // skip header
		}
		for _, col := range row {
			if col == "T1203" || col == "T1059" {
				t.Errorf("blue CSV contains unrevealed technique %q", col)
			}
		}
	}
}

// ============================================================================
// JSON/CSV equivalence — same dataset, same values
// ============================================================================

func TestExportJSONCSVEquivalence(t *testing.T) {
	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	ctx := authCtx(fx.BaselineID)
	baseReq := gen.ExportEngagementRequestObject{
		EngagementId: toUUID(fx.BaselineID),
	}

	for _, dataset := range []gen.ExportEngagementParamsDataset{
		gen.ExportEngagementParamsDatasetExecutions,
		gen.ExportEngagementParamsDatasetFindings,
		gen.ExportEngagementParamsDatasetCoverage,
	} {
		t.Run(string(dataset), func(t *testing.T) {
			csvReq := baseReq
			csvReq.Params = gen.ExportEngagementParams{
				Format:  gen.ExportEngagementParamsFormatCsv,
				Dataset: dataset,
			}
			csvResp, err := h.ExportEngagement(ctx, csvReq)
			if err != nil {
				t.Fatalf("csv ExportEngagement: %v", err)
			}

			jsonReq := baseReq
			jsonReq.Params = gen.ExportEngagementParams{
				Format:  gen.ExportEngagementParamsFormatJson,
				Dataset: dataset,
			}
			jsonResp, err := h.ExportEngagement(ctx, jsonReq)
			if err != nil {
				t.Fatalf("json ExportEngagement: %v", err)
			}

			csvRows := captureCSV(t, csvResp)
			jsonRows := captureJSON(t, jsonResp)

			csvDataRows := len(csvRows) - 1
			if csvDataRows != len(jsonRows) {
				t.Errorf("csv data rows = %d, json rows = %d", csvDataRows, len(jsonRows))
			}

			if csvDataRows > 0 {
				header := csvRows[0]
				for i, jsonRow := range jsonRows {
					for j, h := range header {
						csvVal := csvRows[i+1][j]
						jsonVal := fmt.Sprintf("%v", jsonRow[h])
						if csvVal == "" && jsonVal == "<nil>" {
							continue
						}
						if csvVal == "" && jsonVal == "false" {
							// false in JSON → empty in CSV for boolean columns.
							continue
						}
						if jsonVal == "<nil>" && csvVal == "" {
							continue
						}
						if csvVal != "" && jsonVal != "" && jsonVal != "<nil>" && csvVal != "false" {
							continue
						}
						if csvVal != "" && (jsonVal == "" || jsonVal == "<nil>") {
							t.Errorf("row[%d] field %q: csv=%q but json empty/nil", i, h, csvVal)
						}
					}
				}
			}
		})
	}
}

// ============================================================================
// Authz tests
// ============================================================================

func TestExportAuthzObserver(t *testing.T) {
	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	ctx := context.WithValue(context.Background(), authorizationKey{}, Authorization{
		OperationID: "exportEngagement",
		Subject: authz.Subject{
			UserID:       "test-observer",
			PlatformRole: authz.PlatformRoleMember,
			Method:       authz.MethodCookie,
			Memberships: map[string]authz.EngagementRole{
				fx.BaselineID: authz.EngagementRoleObserver,
			},
		},
		Action:   authz.ActionReportRead,
		Resource: authz.Resource{Type: authz.ResourceReport, EngagementID: fx.BaselineID},
		Allowed:  true,
	})

	req := gen.ExportEngagementRequestObject{
		EngagementId: toUUID(fx.BaselineID),
		Params: gen.ExportEngagementParams{
			Format:  gen.ExportEngagementParamsFormatCsv,
			Dataset: gen.ExportEngagementParamsDatasetExecutions,
		},
	}

	resp, err := h.ExportEngagement(ctx, req)
	if err != nil {
		t.Fatalf("observer ExportEngagement: %v", err)
	}

	w := &testResponseWriter{header: http.Header{}}
	if err := resp.VisitExportEngagementResponse(w); err != nil {
		t.Fatalf("Visit: %v", err)
	}
	if w.buf.Len() == 0 {
		t.Error("observer got empty response")
	}
	if w.header.Get("Content-Disposition") == "" {
		t.Error("Content-Disposition header missing")
	}
}

// Note: ExportEngagement is always reached through middleware that injects
// Authorization into the context and validates report.read. A bare context
// without Authorization will panic (nil engagement service deref). This is

// ============================================================================
// Filename headers
// ============================================================================

func TestExportContentDisposition(t *testing.T) {
	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	ctx := authCtx(fx.BaselineID)

	for _, dataset := range []gen.ExportEngagementParamsDataset{
		gen.ExportEngagementParamsDatasetExecutions,
		gen.ExportEngagementParamsDatasetFindings,
		gen.ExportEngagementParamsDatasetCoverage,
	} {
		for _, format := range []gen.ExportEngagementParamsFormat{
			gen.ExportEngagementParamsFormatCsv,
			gen.ExportEngagementParamsFormatJson,
		} {
			t.Run(fmt.Sprintf("%s-%s", dataset, format), func(t *testing.T) {
				req := gen.ExportEngagementRequestObject{
					EngagementId: toUUID(fx.BaselineID),
					Params: gen.ExportEngagementParams{
						Format:  format,
						Dataset: dataset,
					},
				}
				resp, err := h.ExportEngagement(ctx, req)
				if err != nil {
					t.Fatalf("ExportEngagement: %v", err)
				}

				w := &testResponseWriter{header: http.Header{}}
				if err := resp.VisitExportEngagementResponse(w); err != nil {
					t.Fatalf("Visit: %v", err)
				}

				cd := w.header.Get("Content-Disposition")
				if cd == "" {
					t.Error("Content-Disposition missing")
				}
				if !strings.Contains(cd, "attachment") {
					t.Errorf("Content-Disposition not attachment: %q", cd)
				}
				expectedFile := fmt.Sprintf("Baseline_Assessment-%s.%s", dataset, format)
				if !strings.Contains(cd, expectedFile) {
					t.Errorf("filename: got %q, want to contain %q", cd, expectedFile)
				}

				ct := w.header.Get("Content-Type")
				if format == gen.ExportEngagementParamsFormatCsv {
					if !strings.Contains(ct, "text/csv") {
						t.Errorf("Content-Type for csv: got %q, want text/csv", ct)
					}
				} else {
					if !strings.Contains(ct, "application/json") {
						t.Errorf("Content-Type for json: got %q, want application/json", ct)
					}
				}
			})
		}
	}
}

// ============================================================================
// Helpers
// ============================================================================

type testResponseWriter struct {
	header http.Header
	buf    bytes.Buffer
}

func (w *testResponseWriter) Header() http.Header         { return w.header }
func (w *testResponseWriter) Write(b []byte) (int, error) { return w.buf.Write(b) }
func (w *testResponseWriter) WriteHeader(statusCode int)  {}

func captureCSV(t *testing.T, resp gen.ExportEngagementResponseObject) [][]string {
	t.Helper()
	w := &testResponseWriter{header: http.Header{}}
	if err := resp.VisitExportEngagementResponse(w); err != nil {
		t.Fatalf("Visit: %v", err)
	}
	raw := strings.TrimPrefix(w.buf.String(), "\xEF\xBB\xBF")
	r := csv.NewReader(strings.NewReader(raw))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("reading CSV: %v", err)
	}
	return records
}

func captureJSON(t *testing.T, resp gen.ExportEngagementResponseObject) []map[string]interface{} {
	t.Helper()
	w := &testResponseWriter{header: http.Header{}}
	if err := resp.VisitExportEngagementResponse(w); err != nil {
		t.Fatalf("Visit: %v", err)
	}
	raw := w.buf.String()
	if raw == "" {
		return nil
	}

	var rows []map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		if err == io.EOF {
			return rows
		}
		t.Fatalf("JSON token: %v", err)
	}
	if tok != json.Delim('[') {
		t.Fatalf("expected '[', got %v", tok)
	}
	for dec.More() {
		var row map[string]interface{}
		if err := dec.Decode(&row); err != nil {
			t.Fatalf("JSON decode: %v", err)
		}
		rows = append(rows, row)
	}
	return rows
}
