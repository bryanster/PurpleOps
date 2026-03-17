package main

import (
	"context"
	"html"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// --- Embedded document types ---

type Source struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description,omitempty" json:"description"`
}

type Target struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description,omitempty" json:"description"`
}

type Tool struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description,omitempty" json:"description"`
}

type Control struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description,omitempty" json:"description"`
}

type Tag struct {
	ID     bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name   string        `bson:"name" json:"name"`
	Colour string        `bson:"colour,omitempty" json:"colour"`
}

type Datasource struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description,omitempty" json:"description"`
}

type DetectionRule struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description,omitempty" json:"description"`
}

type FileDoc struct {
	Name    string `bson:"name" json:"name"`
	Path    string `bson:"path" json:"path"`
	Caption string `bson:"caption,omitempty" json:"caption"`
}

// --- Top-level document types ---

type Tactic struct {
	ID      bson.ObjectID `bson:"_id,omitempty" json:"id"`
	MitreID string        `bson:"mitreid" json:"mitreid"`
	Name    string        `bson:"name" json:"name"`
}

type Technique struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	MitreID     string        `bson:"mitreid" json:"mitreid"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description" json:"description"`
	Detection   string        `bson:"detection" json:"detection"`
	Tactics     []string      `bson:"tactics" json:"tactics"`
}

type KnowledgeBase struct {
	ID       bson.ObjectID `bson:"_id,omitempty" json:"id"`
	MitreID  string        `bson:"mitreid" json:"mitreid"`
	Overview string        `bson:"overview" json:"overview"`
	Advice   string        `bson:"advice" json:"advice"`
	Provider string        `bson:"provider" json:"provider"`
}

type Sigma struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	MitreID     string        `bson:"mitreid" json:"mitreid"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description" json:"description"`
	URL         string        `bson:"url" json:"url"`
}

type TestCaseTemplate struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string        `bson:"name" json:"name"`
	MitreID   string        `bson:"mitreid,omitempty" json:"mitreid"`
	Tactic    string        `bson:"tactic,omitempty" json:"tactic"`
	Objective string        `bson:"objective,omitempty" json:"objective"`
	Actions   string        `bson:"actions,omitempty" json:"actions"`
	RedNotes  string        `bson:"rednotes,omitempty" json:"rednotes"`
	UUID      string        `bson:"uuid,omitempty" json:"uuid"`
	Provider  string        `bson:"provider,omitempty" json:"provider"`
}

type TestCase struct {
	ID               bson.ObjectID `bson:"_id,omitempty" json:"id"`
	AssessmentID     string        `bson:"assessmentid" json:"assessmentid"`
	Name             string        `bson:"name" json:"name"`
	Objective        string        `bson:"objective,omitempty" json:"objective"`
	Actions          string        `bson:"actions,omitempty" json:"actions"`
	RedNotes         string        `bson:"rednotes,omitempty" json:"rednotes"`
	BlueNotes        string        `bson:"bluenotes,omitempty" json:"bluenotes"`
	UUID             string        `bson:"uuid,omitempty" json:"uuid"`
	MitreID          string        `bson:"mitreid" json:"mitreid"`
	Tactic           string        `bson:"tactic" json:"tactic"`
	Sources          []string      `bson:"sources,omitempty" json:"sources"`
	Targets          []string      `bson:"targets,omitempty" json:"targets"`
	Tools            []string      `bson:"tools,omitempty" json:"tools"`
	Controls         []string      `bson:"controls,omitempty" json:"controls"`
	Tags             []string      `bson:"tags,omitempty" json:"tags"`
	Datasources      []string      `bson:"datasources,omitempty" json:"datasources"`
	Rules            []string      `bson:"rules,omitempty" json:"rules"`
	DetectionSource  string        `bson:"detectionsource,omitempty" json:"detectionsource"`
	PreventionSource string        `bson:"preventionsource,omitempty" json:"preventionsource"`
	State            string        `bson:"state,omitempty" json:"state"`
	Prevented        string        `bson:"prevented,omitempty" json:"prevented"`
	PreventedRating  string        `bson:"preventedrating,omitempty" json:"preventedrating"`
	Alerted          *bool         `bson:"alerted,omitempty" json:"alerted"`
	AlertSeverity    string        `bson:"alertseverity,omitempty" json:"alertseverity"`
	Logged           *bool         `bson:"logged,omitempty" json:"logged"`
	DetectionRating  string        `bson:"detectionrating,omitempty" json:"detectionrating"`
	Priority         string        `bson:"priority,omitempty" json:"priority"`
	PriorityUrgency  string        `bson:"priorityurgency,omitempty" json:"priorityurgency"`
	StartTime        *time.Time    `bson:"starttime,omitempty" json:"starttime"`
	EndTime          *time.Time    `bson:"endtime,omitempty" json:"endtime"`
	DetectTime       *time.Time    `bson:"detecttime,omitempty" json:"detecttime"`
	RedFiles         []FileDoc     `bson:"redfiles,omitempty" json:"redfiles"`
	BlueFiles        []FileDoc     `bson:"bluefiles,omitempty" json:"bluefiles"`
	Visible          bool          `bson:"visible" json:"visible"`
	ModifyTime       *time.Time    `bson:"modifytime,omitempty" json:"modifytime"`
	Outcome          string        `bson:"outcome,omitempty" json:"outcome"`
}

type Assessment struct {
	ID                bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name              string        `bson:"name" json:"name"`
	Description       string        `bson:"description,omitempty" json:"description"`
	Created           *time.Time    `bson:"created,omitempty" json:"created"`
	Targets           []Target      `bson:"targets,omitempty" json:"targets"`
	Sources           []Source      `bson:"sources,omitempty" json:"sources"`
	Tools             []Tool        `bson:"tools,omitempty" json:"tools"`
	Controls          []Control     `bson:"controls,omitempty" json:"controls"`
	Tags              []Tag         `bson:"tags,omitempty" json:"tags"`
	Datasources       []Datasource  `bson:"datasources,omitempty" json:"datasources"`
	Rules             []DetectionRule `bson:"rules,omitempty" json:"rules"`
	DetectionSources  []Datasource  `bson:"detectionsources,omitempty" json:"detectionsources"`
	PreventionSources []Datasource  `bson:"preventionsources,omitempty" json:"preventionsources"`
	NavigatorExport   string        `bson:"navigatorexport,omitempty" json:"navigatorexport"`
}

type Role struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description,omitempty" json:"description"`
}

type User struct {
	ID             bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	Email          string          `bson:"email" json:"email"`
	Username       string          `bson:"username" json:"username"`
	Password       string          `bson:"password" json:"-"`
	Roles          []bson.ObjectID `bson:"roles,omitempty" json:"-"`
	Assessments    []bson.ObjectID `bson:"assessments,omitempty" json:"-"`
	InitPwd        bool            `bson:"initpwd" json:"initpwd"`
	Active         bool            `bson:"active" json:"active"`
	LastLoginAt    *time.Time      `bson:"last_login_at,omitempty" json:"last_login_at"`
	CurrentLoginAt *time.Time      `bson:"current_login_at,omitempty" json:"current_login_at"`
	LastLoginIP    string          `bson:"last_login_ip,omitempty" json:"last_login_ip"`
	CurrentLoginIP string          `bson:"current_login_ip,omitempty" json:"current_login_ip"`
	LoginCount     int             `bson:"login_count,omitempty" json:"login_count"`
	TFMethod       string          `bson:"tf_primary_method,omitempty" json:"-"`
	TFSecret       string          `bson:"tf_totp_secret,omitempty" json:"-"`
}

// --- Helper functions ---

func esc(s string, raw bool) string {
	if raw {
		return s
	}
	return html.EscapeString(s)
}

func timeStr(t *time.Time) string {
	if t == nil {
		return "None"
	}
	return t.Format("2006-01-02 15:04:05")
}

func timeStrLocal(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02T15:04")
}

func boolPtr(b bool) *bool {
	return &b
}

func nowPtr() *time.Time {
	t := time.Now().UTC()
	return &t
}

// --- JSON serialization methods ---

func (tc *TestCase) ToJSON(raw bool) map[string]interface{} {
	j := map[string]interface{}{
		"assessmentid":    esc(tc.AssessmentID, raw),
		"name":            esc(tc.Name, raw),
		"objective":       esc(tc.Objective, raw),
		"actions":         esc(tc.Actions, raw),
		"rednotes":        esc(tc.RedNotes, raw),
		"bluenotes":       esc(tc.BlueNotes, raw),
		"uuid":            esc(tc.UUID, raw),
		"mitreid":         esc(tc.MitreID, raw),
		"tactic":          esc(tc.Tactic, raw),
		"state":           esc(tc.State, raw),
		"prevented":       esc(tc.Prevented, raw),
		"preventedrating": esc(tc.PreventedRating, raw),
		"alerted":         tc.Alerted,
		"alertseverity":   esc(tc.AlertSeverity, raw),
		"logged":          tc.Logged,
		"detectionrating": esc(tc.DetectionRating, raw),
		"priority":        esc(tc.Priority, raw),
		"priorityurgency": esc(tc.PriorityUrgency, raw),
		"visible":         tc.Visible,
		"outcome":         esc(tc.Outcome, raw),
		"detectionsource": esc(tc.DetectionSource, raw),
		"preventionsource": esc(tc.PreventionSource, raw),
		"id":              tc.ID.Hex(),
		"detecttime":      timeStr(tc.DetectTime),
		"modifytime":      timeStr(tc.ModifyTime),
		"starttime":       timeStr(tc.StartTime),
		"endtime":         timeStr(tc.EndTime),
	}

	// Multi fields - resolve to names
	for _, field := range []string{"tags", "sources", "targets", "tools", "controls", "datasources", "rules"} {
		j[field] = tc.ToJSONMulti(field)
	}

	// File fields
	for _, field := range []string{"redfiles", "bluefiles"} {
		var files []FileDoc
		if field == "redfiles" {
			files = tc.RedFiles
		} else {
			files = tc.BlueFiles
		}
		strs := make([]string, 0, len(files))
		for _, f := range files {
			strs = append(strs, f.Path+"|"+f.Caption)
		}
		j[field] = strs
	}

	return j
}

func (tc *TestCase) ToJSONMulti(field string) []string {
	ctx := context.Background()
	oid, err := bson.ObjectIDFromHex(tc.AssessmentID)
	if err != nil {
		return []string{}
	}
	var assessment Assessment
	if err := Col("assessment").FindOne(ctx, bson.M{"_id": oid}).Decode(&assessment); err != nil {
		return []string{}
	}

	var ids []string
	switch field {
	case "sources":
		ids = tc.Sources
	case "targets":
		ids = tc.Targets
	case "tools":
		ids = tc.Tools
	case "controls":
		ids = tc.Controls
	case "tags":
		ids = tc.Tags
	case "datasources":
		ids = tc.Datasources
	case "rules":
		ids = tc.Rules
	}

	strs := make([]string, 0, len(ids))
	for _, id := range ids {
		switch field {
		case "tags":
			for _, t := range assessment.Tags {
				if t.ID.Hex() == id {
					strs = append(strs, t.Name+"|"+t.Colour)
				}
			}
		case "sources":
			for _, s := range assessment.Sources {
				if s.ID.Hex() == id {
					strs = append(strs, s.Name+"|"+s.Description)
				}
			}
		case "targets":
			for _, t := range assessment.Targets {
				if t.ID.Hex() == id {
					strs = append(strs, t.Name+"|"+t.Description)
				}
			}
		case "tools":
			for _, t := range assessment.Tools {
				if t.ID.Hex() == id {
					strs = append(strs, t.Name+"|"+t.Description)
				}
			}
		case "controls":
			for _, c := range assessment.Controls {
				if c.ID.Hex() == id {
					strs = append(strs, c.Name+"|"+c.Description)
				}
			}
		case "datasources":
			for _, d := range assessment.Datasources {
				if d.ID.Hex() == id {
					strs = append(strs, d.Name+"|"+d.Description)
				}
			}
		case "rules":
			for _, r := range assessment.Rules {
				if r.ID.Hex() == id {
					strs = append(strs, r.Name+"|"+r.Description)
				}
			}
		}
	}
	return strs
}

func (a *Assessment) GetProgress() string {
	ctx := context.Background()
	total, _ := Col("test_case").CountDocuments(ctx, bson.M{"assessmentid": a.ID.Hex()})
	if total == 0 {
		return "0|0|0|0|0"
	}

	results := make([]string, 0, 4)
	for _, outcome := range []string{"Prevented", "Alerted", "Logged", "Missed"} {
		count, _ := Col("test_case").CountDocuments(ctx, bson.M{
			"assessmentid": a.ID.Hex(),
			"outcome":      outcome,
		})
		pct := float64(count) / float64(total) * 100
		results = append(results, formatFloat(pct))
	}
	return strings.Join(results, "|")
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

func (a *Assessment) ToJSON(raw bool) map[string]interface{} {
	return map[string]interface{}{
		"id":          a.ID.Hex(),
		"name":        esc(a.Name, raw),
		"description": esc(a.Description, raw),
		"progress":    a.GetProgress(),
		"created":     timeStr(a.Created),
	}
}

func (a *Assessment) MultiToJSON(field string, raw bool) []map[string]interface{} {
	var result []map[string]interface{}
	switch field {
	case "sources":
		for _, s := range a.Sources {
			result = append(result, map[string]interface{}{
				"id": s.ID.Hex(), "name": esc(s.Name, raw), "description": esc(s.Description, raw),
			})
		}
	case "targets":
		for _, t := range a.Targets {
			result = append(result, map[string]interface{}{
				"id": t.ID.Hex(), "name": esc(t.Name, raw), "description": esc(t.Description, raw),
			})
		}
	case "tools":
		for _, t := range a.Tools {
			result = append(result, map[string]interface{}{
				"id": t.ID.Hex(), "name": esc(t.Name, raw), "description": esc(t.Description, raw),
			})
		}
	case "controls":
		for _, c := range a.Controls {
			result = append(result, map[string]interface{}{
				"id": c.ID.Hex(), "name": esc(c.Name, raw), "description": esc(c.Description, raw),
			})
		}
	case "tags":
		for _, t := range a.Tags {
			result = append(result, map[string]interface{}{
				"id": t.ID.Hex(), "name": esc(t.Name, raw), "colour": esc(t.Colour, raw),
			})
		}
	case "datasources":
		for _, d := range a.Datasources {
			result = append(result, map[string]interface{}{
				"id": d.ID.Hex(), "name": esc(d.Name, raw), "description": esc(d.Description, raw),
			})
		}
	case "rules":
		for _, r := range a.Rules {
			result = append(result, map[string]interface{}{
				"id": r.ID.Hex(), "name": esc(r.Name, raw), "description": esc(r.Description, raw),
			})
		}
	case "detectionsources":
		for _, d := range a.DetectionSources {
			result = append(result, map[string]interface{}{
				"id": d.ID.Hex(), "name": esc(d.Name, raw), "description": esc(d.Description, raw),
			})
		}
	case "preventionsources":
		for _, p := range a.PreventionSources {
			result = append(result, map[string]interface{}{
				"id": p.ID.Hex(), "name": esc(p.Name, raw), "description": esc(p.Description, raw),
			})
		}
	}
	return result
}

// GetRoleNames resolves role IDs to role names
func (u *User) GetRoleNames(ctx context.Context) []string {
	names := make([]string, 0, len(u.Roles))
	for _, rid := range u.Roles {
		var role Role
		if err := Col("role").FindOne(ctx, bson.M{"_id": rid}).Decode(&role); err == nil {
			names = append(names, role.Name)
		}
	}
	return names
}

func (u *User) HasRole(ctx context.Context, roleName string) bool {
	for _, name := range u.GetRoleNames(ctx) {
		if name == roleName {
			return true
		}
	}
	return false
}

func (u *User) GetAssessmentNames(ctx context.Context) []string {
	names := make([]string, 0, len(u.Assessments))
	for _, aid := range u.Assessments {
		var a Assessment
		if err := Col("assessment").FindOne(ctx, bson.M{"_id": aid}).Decode(&a); err == nil {
			names = append(names, a.Name)
		}
	}
	return names
}

func (u *User) AssessmentList(ctx context.Context) []bson.ObjectID {
	if u.HasRole(ctx, "Admin") {
		var assessments []Assessment
		cursor, err := Col("assessment").Find(ctx, bson.M{})
		if err != nil {
			return nil
		}
		if err := cursor.All(ctx, &assessments); err != nil {
			return nil
		}
		ids := make([]bson.ObjectID, len(assessments))
		for i, a := range assessments {
			ids[i] = a.ID
		}
		return ids
	}
	return u.Assessments
}

func (u *User) ToJSON(ctx context.Context, raw bool) map[string]interface{} {
	return map[string]interface{}{
		"id":               u.ID.Hex(),
		"username":         esc(u.Username, raw),
		"email":            esc(u.Email, raw),
		"roles":            u.GetRoleNames(ctx),
		"assessments":      u.GetAssessmentNames(ctx),
		"current_login_at": u.CurrentLoginAt,
		"current_login_ip": u.CurrentLoginIP,
	}
}

// --- DB query helpers ---

func FindAssessment(ctx context.Context, id string) (*Assessment, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var a Assessment
	err = Col("assessment").FindOne(ctx, bson.M{"_id": oid}).Decode(&a)
	return &a, err
}

func FindTestCase(ctx context.Context, id string) (*TestCase, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var tc TestCase
	err = Col("test_case").FindOne(ctx, bson.M{"_id": oid}).Decode(&tc)
	return &tc, err
}

func FindUser(ctx context.Context, id string) (*User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var u User
	err = Col("user").FindOne(ctx, bson.M{"_id": oid}).Decode(&u)
	return &u, err
}

func FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := Col("user").FindOne(ctx, bson.M{"email": email}).Decode(&u)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &u, err
}

func FindUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := Col("user").FindOne(ctx, bson.M{"username": username}).Decode(&u)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &u, err
}

func FindRole(ctx context.Context, name string) (*Role, error) {
	var r Role
	err := Col("role").FindOne(ctx, bson.M{"name": name}).Decode(&r)
	return &r, err
}

func GetTestCases(ctx context.Context, assessmentID string) ([]TestCase, error) {
	var tcs []TestCase
	cursor, err := Col("test_case").Find(ctx, bson.M{"assessmentid": assessmentID})
	if err != nil {
		return nil, err
	}
	if err := cursor.All(ctx, &tcs); err != nil {
		return nil, err
	}
	return tcs, nil
}
