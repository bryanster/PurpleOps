package models

import (
	"context"
	"strings"

	"github.com/bryanster/purpleops/internal/db"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ToJSON serializes a TestCase to a template/JSON-friendly map.
func (tc *TestCase) ToJSON(raw bool) map[string]interface{} {
	j := map[string]interface{}{
		"assessmentid":     Esc(tc.AssessmentID, raw),
		"name":             Esc(tc.Name, raw),
		"objective":        Esc(tc.Objective, raw),
		"actions":          Esc(tc.Actions, raw),
		"rednotes":         Esc(tc.RedNotes, raw),
		"bluenotes":        Esc(tc.BlueNotes, raw),
		"uuid":             Esc(tc.UUID, raw),
		"mitreid":          Esc(tc.MitreID, raw),
		"tactic":           Esc(tc.Tactic, raw),
		"state":            Esc(tc.State, raw),
		"prevented":        Esc(tc.Prevented, raw),
		"preventedrating":  Esc(tc.PreventedRating, raw),
		"alerted":          tc.Alerted,
		"alertseverity":    Esc(tc.AlertSeverity, raw),
		"logged":           tc.Logged,
		"detectionrating":  Esc(tc.DetectionRating, raw),
		"priority":         Esc(tc.Priority, raw),
		"priorityurgency":  Esc(tc.PriorityUrgency, raw),
		"visible":          tc.Visible,
		"outcome":          Esc(tc.Outcome, raw),
		"detectionsource":  Esc(tc.DetectionSource, raw),
		"preventionsource": Esc(tc.PreventionSource, raw),
		"id":               tc.ID.Hex(),
		"detecttime":       TimeStr(tc.DetectTime),
		"modifytime":       TimeStr(tc.ModifyTime),
		"starttime":        TimeStr(tc.StartTime),
		"endtime":          TimeStr(tc.EndTime),
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

// ToJSONMulti resolves multi-select field IDs to their human-readable names.
func (tc *TestCase) ToJSONMulti(field string) []string {
	ctx := context.Background()
	oid, err := bson.ObjectIDFromHex(tc.AssessmentID)
	if err != nil {
		return []string{}
	}
	var assessment Assessment
	if err := db.Col(db.ColAssessment).FindOne(ctx, bson.M{"_id": oid}).Decode(&assessment); err != nil {
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

// AlertedNo returns true when the testcase's Alerted field is explicitly false.
func (tc *TestCase) AlertedNo() bool {
	return tc.Alerted != nil && !*tc.Alerted
}

// AlertedNull returns true when the testcase's Alerted field is nil (unset).
func (tc *TestCase) AlertedNull() bool {
	return tc.Alerted == nil
}

// LoggedNo returns true when the testcase's Logged field is explicitly false.
func (tc *TestCase) LoggedNo() bool {
	return tc.Logged != nil && !*tc.Logged
}

// LoggedNull returns true when the testcase's Logged field is nil (unset).
func (tc *TestCase) LoggedNull() bool {
	return tc.Logged == nil
}

// TagsJSON returns the testcase's tags resolved to "name|colour" format,
// comma-separated, for display in the assessment table.
func (tc *TestCase) TagsJSON() string {
	return strings.Join(tc.ToJSONMulti("tags"), ",")
}

// GetProgress returns a "|"-delimited string of outcome percentages for an assessment.
func (a *Assessment) GetProgress() string {
	ctx := context.Background()
	total, _ := db.Col(db.ColTestCase).CountDocuments(ctx, bson.M{"assessmentid": a.ID.Hex()})
	if total == 0 {
		return "0|0|0|0|0"
	}

	results := make([]string, 0, 4)
	for _, outcome := range []string{"Prevented", "Alerted", "Logged", "Missed"} {
		count, _ := db.Col(db.ColTestCase).CountDocuments(ctx, bson.M{
			"assessmentid": a.ID.Hex(),
			"outcome":      outcome,
		})
		pct := float64(count) / float64(total) * 100
		results = append(results, FormatFloat(pct))
	}
	return strings.Join(results, "|")
}

// ToJSON serializes an Assessment to a template/JSON-friendly map.
func (a *Assessment) ToJSON(raw bool) map[string]interface{} {
	return map[string]interface{}{
		"id":          a.ID.Hex(),
		"name":        Esc(a.Name, raw),
		"description": Esc(a.Description, raw),
		"progress":    a.GetProgress(),
		"created":     TimeStr(a.Created),
	}
}

// MultiToJSON serializes a specific embedded list field of an Assessment.
func (a *Assessment) MultiToJSON(field string, raw bool) []map[string]interface{} {
	var result []map[string]interface{}
	switch field {
	case "sources":
		for _, s := range a.Sources {
			result = append(result, map[string]interface{}{
				"id": s.ID.Hex(), "name": Esc(s.Name, raw), "description": Esc(s.Description, raw),
			})
		}
	case "targets":
		for _, t := range a.Targets {
			result = append(result, map[string]interface{}{
				"id": t.ID.Hex(), "name": Esc(t.Name, raw), "description": Esc(t.Description, raw),
			})
		}
	case "tools":
		for _, t := range a.Tools {
			result = append(result, map[string]interface{}{
				"id": t.ID.Hex(), "name": Esc(t.Name, raw), "description": Esc(t.Description, raw),
			})
		}
	case "controls":
		for _, c := range a.Controls {
			result = append(result, map[string]interface{}{
				"id": c.ID.Hex(), "name": Esc(c.Name, raw), "description": Esc(c.Description, raw),
			})
		}
	case "tags":
		for _, t := range a.Tags {
			result = append(result, map[string]interface{}{
				"id": t.ID.Hex(), "name": Esc(t.Name, raw), "colour": Esc(t.Colour, raw),
			})
		}
	case "datasources":
		for _, d := range a.Datasources {
			result = append(result, map[string]interface{}{
				"id": d.ID.Hex(), "name": Esc(d.Name, raw), "description": Esc(d.Description, raw),
			})
		}
	case "rules":
		for _, r := range a.Rules {
			result = append(result, map[string]interface{}{
				"id": r.ID.Hex(), "name": Esc(r.Name, raw), "description": Esc(r.Description, raw),
			})
		}
	case "detectionsources":
		for _, d := range a.DetectionSources {
			result = append(result, map[string]interface{}{
				"id": d.ID.Hex(), "name": Esc(d.Name, raw), "description": Esc(d.Description, raw),
			})
		}
	case "preventionsources":
		for _, p := range a.PreventionSources {
			result = append(result, map[string]interface{}{
				"id": p.ID.Hex(), "name": Esc(p.Name, raw), "description": Esc(p.Description, raw),
			})
		}
	}
	return result
}

// GetRoleNames resolves an APIKey's role IDs to their names.
func (k *APIKey) GetRoleNames(ctx context.Context) []string {
	names := make([]string, 0, len(k.Roles))
	for _, rid := range k.Roles {
		var role Role
		if err := db.Col(db.ColRole).FindOne(ctx, bson.M{"_id": rid}).Decode(&role); err == nil {
			names = append(names, role.Name)
		}
	}
	return names
}

// GetAssessmentNames resolves an APIKey's assessment IDs to their names.
func (k *APIKey) GetAssessmentNames(ctx context.Context) []string {
	names := make([]string, 0, len(k.Assessments))
	for _, aid := range k.Assessments {
		var a Assessment
		if err := db.Col(db.ColAssessment).FindOne(ctx, bson.M{"_id": aid}).Decode(&a); err == nil {
			names = append(names, a.Name)
		}
	}
	return names
}

// GetRoleNames resolves the user's role IDs to their names.
func (u *User) GetRoleNames(ctx context.Context) []string {
	names := make([]string, 0, len(u.Roles))
	for _, rid := range u.Roles {
		var role Role
		if err := db.Col(db.ColRole).FindOne(ctx, bson.M{"_id": rid}).Decode(&role); err == nil {
			names = append(names, role.Name)
		}
	}
	return names
}

// HasRole checks whether the user has the named role.
func (u *User) HasRole(ctx context.Context, roleName string) bool {
	for _, name := range u.GetRoleNames(ctx) {
		if name == roleName {
			return true
		}
	}
	return false
}

// GetAssessmentNames returns the names of the assessments the user is assigned to.
func (u *User) GetAssessmentNames(ctx context.Context) []string {
	names := make([]string, 0, len(u.Assessments))
	for _, aid := range u.Assessments {
		var a Assessment
		if err := db.Col(db.ColAssessment).FindOne(ctx, bson.M{"_id": aid}).Decode(&a); err == nil {
			names = append(names, a.Name)
		}
	}
	return names
}

// AssessmentList returns the list of assessment IDs the user can access.
// Admins see all assessments.
func (u *User) AssessmentList(ctx context.Context) []bson.ObjectID {
	if u.HasRole(ctx, "Admin") {
		var assessments []Assessment
		cursor, err := db.Col(db.ColAssessment).Find(ctx, bson.M{})
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

// ToJSON serializes a User to a template/JSON-friendly map.
func (u *User) ToJSON(ctx context.Context, raw bool) map[string]interface{} {
	return map[string]interface{}{
		"id":               u.ID.Hex(),
		"username":         Esc(u.Username, raw),
		"email":            Esc(u.Email, raw),
		"roles":            u.GetRoleNames(ctx),
		"assessments":      u.GetAssessmentNames(ctx),
		"current_login_at": u.CurrentLoginAt,
		"current_login_ip": u.CurrentLoginIP,
	}
}
