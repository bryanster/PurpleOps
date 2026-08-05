package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/activity"
	"github.com/bryanster/blacklight/internal/store/identity"
)

const (
	customProceduresPath = BasePath + "/content/custom/procedure-templates"
	customDetectionsPath = BasePath + "/content/custom/detection-rules"
	customNotesPath      = BasePath + "/content/custom/notes"
	customExportPath     = BasePath + "/content/custom/export"
)

func TestMemberCanReadCustomButNotManage(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t) // admin
	member := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "member-custom@example.com"
		in.DisplayName = "Member"
		in.PlatformRole = authz.PlatformRoleMember
	})
	memberSession := sessionCookie(t, server.login(member.Email, testPassword))
	admin := server.signIn(t)

	// Seed one of each type as admin so reads have something to see.
	created := server.post(customProceduresPath, `{
		"name":"Seed proc",
		"command":"echo hi",
		"cleanup":"rm -f /tmp/x",
		"inputArgs":[{"name":"path","description":"p","type":"path","default":"/tmp"}],
		"techniqueExternalIds":["T1059.001"]
	}`, admin)
	if created.Code != http.StatusCreated {
		t.Fatalf("seed procedure: %d\n%s", created.Code, created.Body)
	}
	var proc gen.ContentProcedureTemplate
	if err := json.Unmarshal(created.Body.Bytes(), &proc); err != nil {
		t.Fatal(err)
	}

	// Member can list/get.
	if got := server.get(customProceduresPath, memberSession); got.Code != http.StatusOK {
		t.Fatalf("member list procedures: %d", got.Code)
	}
	if got := server.get(customProceduresPath+"/"+proc.Id.String(), memberSession); got.Code != http.StatusOK {
		t.Fatalf("member get procedure: %d", got.Code)
	}

	// Member cannot mutate.
	if got := server.post(customProceduresPath, `{"name":"nope"}`, memberSession); got.Code != http.StatusForbidden {
		t.Fatalf("member create: %d, want 403", got.Code)
	}
	if got := server.send(http.MethodPatch, customProceduresPath+"/"+proc.Id.String(), `{"name":"x"}`, memberSession); got.Code != http.StatusForbidden {
		t.Fatalf("member patch: %d, want 403", got.Code)
	}
	if got := server.send(http.MethodDelete, customProceduresPath+"/"+proc.Id.String(), "", memberSession); got.Code != http.StatusForbidden {
		t.Fatalf("member delete: %d, want 403", got.Code)
	}
}

func TestCustomProcedureTemplateCRUDPreservesStructure(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	createBody := `{
		"name":"PowerShell spawn",
		"description":"runs a command",
		"platforms":["windows"],
		"executor":"powershell",
		"elevationRequired":true,
		"command":"Write-Host #{msg}",
		"cleanup":"Remove-Item x",
		"inputArgs":[{"name":"msg","description":"message","type":"string","default":"hi"}],
		"techniqueExternalIds":["T1059.001"]
	}`
	created := server.post(customProceduresPath, createBody, admin)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d\n%s", created.Code, created.Body)
	}
	var proc gen.ContentProcedureTemplate
	if err := json.Unmarshal(created.Body.Bytes(), &proc); err != nil {
		t.Fatal(err)
	}
	if proc.Name != "PowerShell spawn" || proc.Command == "" || proc.Cleanup == "" {
		t.Fatalf("structure lost: %+v", proc)
	}
	if len(proc.InputArgs) != 1 || proc.InputArgs[0].Name != "msg" {
		t.Fatalf("inputArgs = %+v", proc.InputArgs)
	}
	if len(proc.TechniqueExternalIds) != 1 || proc.TechniqueExternalIds[0] != "T1059.001" {
		t.Fatalf("techniques = %+v", proc.TechniqueExternalIds)
	}
	if !proc.ElevationRequired || proc.Executor != "powershell" {
		t.Fatalf("executor/elevation: %+v", proc)
	}

	// Invalid technique id → 400 field error.
	bad := server.post(customProceduresPath, `{"name":"x","techniqueExternalIds":["not-a-tech"]}`, admin)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid technique: %d, want 400\n%s", bad.Code, bad.Body)
	}
	problem := decodeProblem(t, bad)
	if problem.Code != gen.ProblemCodeValidationFailed {
		t.Fatalf("code = %q", problem.Code)
	}
	if problem.Errors == nil || len(*problem.Errors) == 0 {
		t.Fatal("expected field errors")
	}

	// Patch name.
	patched := server.send(http.MethodPatch, customProceduresPath+"/"+proc.Id.String(), `{"name":"Renamed"}`, admin)
	if patched.Code != http.StatusOK {
		t.Fatalf("patch: %d\n%s", patched.Code, patched.Body)
	}
	var updated gen.ContentProcedureTemplate
	if err := json.Unmarshal(patched.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Renamed" || updated.Command != proc.Command {
		t.Fatalf("patch changed more than name: %+v", updated)
	}

	// Delete + activity.
	del := server.send(http.MethodDelete, customProceduresPath+"/"+proc.Id.String(), "", admin)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete: %d\n%s", del.Code, del.Body)
	}
	if got := server.get(customProceduresPath+"/"+proc.Id.String(), admin); got.Code != http.StatusNotFound {
		t.Fatalf("get after delete: %d", got.Code)
	}

	entries, _, err := activity.New(server.db).List(t.Context(), activity.ListFilter{
		ScopePlatform: true,
		Verb:          string(events.VerbContentCustomDeleted),
		Limit:         10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected content.custom.deleted activity row")
	}
	if entries[0].ObjectType != events.ObjectContentProcedureTemplate {
		t.Fatalf("object type = %q", entries[0].ObjectType)
	}

}

func TestCustomDetectionAndNoteCRUD(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	det := server.post(customDetectionsPath, `{
		"name":"Custom high rule",
		"ruleYaml":"title: Custom\ndetection: {}\n",
		"level":"high",
		"status":"experimental",
		"logsource":{"product":"windows"},
		"techniqueExternalIds":["T1059"]
	}`, admin)
	if det.Code != http.StatusCreated {
		t.Fatalf("create detection: %d\n%s", det.Code, det.Body)
	}
	var rule gen.ContentDetectionRule
	if err := json.Unmarshal(det.Body.Bytes(), &rule); err != nil {
		t.Fatal(err)
	}
	if rule.RuleYaml == "" || rule.Level != "high" {
		t.Fatalf("detection: %+v", rule)
	}

	note := server.post(customNotesPath, `{
		"title":"KB note",
		"bodyMarkdown":"Be careful with PowerShell.",
		"tags":["windows"],
		"techniqueExternalId":"T1059.001"
	}`, admin)
	if note.Code != http.StatusCreated {
		t.Fatalf("create note: %d\n%s", note.Code, note.Body)
	}
	var n gen.ContentNote
	if err := json.Unmarshal(note.Body.Bytes(), &n); err != nil {
		t.Fatal(err)
	}
	if n.Title != "KB note" || n.BodyMarkdown == "" {
		t.Fatalf("note: %+v", n)
	}

	// Invalid technique on note.
	badNote := server.post(customNotesPath, `{"title":"x","bodyMarkdown":"y","techniqueExternalId":"X1"}`, admin)
	if badNote.Code != http.StatusBadRequest {
		t.Fatalf("bad technique note: %d", badNote.Code)
	}

	// Delete both; activity for note delete.
	if got := server.send(http.MethodDelete, customDetectionsPath+"/"+rule.Id.String(), "", admin); got.Code != http.StatusNoContent {
		t.Fatalf("delete detection: %d", got.Code)
	}
	if got := server.send(http.MethodDelete, customNotesPath+"/"+n.Id.String(), "", admin); got.Code != http.StatusNoContent {
		t.Fatalf("delete note: %d", got.Code)
	}
	entries, _, err := activity.New(server.db).List(t.Context(), activity.ListFilter{
		ScopePlatform: true,
		Verb:          string(events.VerbContentCustomDeleted),
		Limit:         20,
	})
	if err != nil {
		t.Fatal(err)
	}
	var sawNote bool
	for _, e := range entries {
		if e.ObjectType == events.ObjectContentNote {
			sawNote = true
		}
	}
	if !sawNote {
		t.Fatal("expected note delete activity")
	}

}

func TestCustomExportRoundTripJSONAndYAML(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	if got := server.post(customProceduresPath, `{"name":"P1","command":"echo 1","techniqueExternalIds":["T1003"]}`, admin); got.Code != http.StatusCreated {
		t.Fatalf("proc: %d\n%s", got.Code, got.Body)
	}
	if got := server.post(customDetectionsPath, `{"name":"D1","ruleYaml":"title: D1\n"}`, admin); got.Code != http.StatusCreated {
		t.Fatalf("det: %d\n%s", got.Code, got.Body)
	}
	if got := server.post(customNotesPath, `{"title":"N1","bodyMarkdown":"body"}`, admin); got.Code != http.StatusCreated {
		t.Fatalf("note: %d\n%s", got.Code, got.Body)
	}

	// JSON export.
	jsonResp := server.get(customExportPath+"?format=json", admin)
	if jsonResp.Code != http.StatusOK {
		t.Fatalf("json export: %d\n%s", jsonResp.Code, jsonResp.Body)
	}
	if ct := jsonResp.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("json content-type = %q", ct)
	}
	var doc gen.ContentCustomExport
	if err := json.Unmarshal(jsonResp.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.ProcedureTemplates) != 1 || len(doc.DetectionRules) != 1 || len(doc.Notes) != 1 {
		t.Fatalf("json counts: procs=%d dets=%d notes=%d",
			len(doc.ProcedureTemplates), len(doc.DetectionRules), len(doc.Notes))
	}
	if doc.Meta.SourceName == "" || doc.Meta.Attribution == "" {
		t.Fatalf("meta incomplete: %+v", doc.Meta)
	}

	// YAML export — header comments + body with all three types.
	yamlResp := server.get(customExportPath+"?format=yaml", admin)
	if yamlResp.Code != http.StatusOK {
		t.Fatalf("yaml export: %d\n%s", yamlResp.Code, yamlResp.Body)
	}
	if ct := yamlResp.Header().Get("Content-Type"); !strings.Contains(ct, "yaml") {
		t.Fatalf("yaml content-type = %q", ct)
	}
	raw := yamlResp.Body.String()
	if !strings.Contains(raw, "# Blacklight custom content export") {
		t.Fatalf("missing header comment:\n%s", raw)
	}
	if !strings.Contains(raw, "# Attribution:") {
		t.Fatalf("missing attribution header:\n%s", raw)
	}
	// Strip comment lines for parse.
	var body strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	var ydoc gen.ContentCustomExport
	if err := yaml.Unmarshal([]byte(body.String()), &ydoc); err != nil {
		t.Fatalf("yaml parse: %v\n%s", err, body.String())
	}
	if len(ydoc.ProcedureTemplates) != 1 || len(ydoc.DetectionRules) != 1 || len(ydoc.Notes) != 1 {
		t.Fatalf("yaml counts: procs=%d dets=%d notes=%d",
			len(ydoc.ProcedureTemplates), len(ydoc.DetectionRules), len(ydoc.Notes))
	}

	// Type filter.
	onlyNotes := server.get(customExportPath+"?format=json&type=notes", admin)
	if onlyNotes.Code != http.StatusOK {
		t.Fatalf("type=notes: %d", onlyNotes.Code)
	}
	var notesOnly gen.ContentCustomExport
	if err := json.Unmarshal(onlyNotes.Body.Bytes(), &notesOnly); err != nil {
		t.Fatal(err)
	}
	if len(notesOnly.Notes) != 1 || len(notesOnly.ProcedureTemplates) != 0 || len(notesOnly.DetectionRules) != 0 {
		t.Fatalf("type filter: %+v", notesOnly)
	}
}

func TestCustomEmptyNameRejected(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	// OpenAPI minLength=1 should catch empty name before the handler.
	got := server.post(customProceduresPath, `{"name":""}`, admin)
	if got.Code != http.StatusBadRequest {
		t.Fatalf("empty name: %d, want 400\n%s", got.Code, got.Body)
	}
}
