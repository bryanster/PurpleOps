package models

import "go.mongodb.org/mongo-driver/v2/bson"

// NamedItem is implemented by all assessment embedded document types
// (Source, Target, Tool, Control, Tag, Datasource, DetectionRule).
type NamedItem interface {
	GetID() bson.ObjectID
	GetName() string
	SetFields(data map[string]string)
}

// --- Source ---

func (s Source) GetID() bson.ObjectID { return s.ID }
func (s Source) GetName() string      { return s.Name }
func (s *Source) SetFields(d map[string]string) {
	s.Name = d["name"]
	s.Description = d["description"]
}

// --- Target ---

func (t Target) GetID() bson.ObjectID { return t.ID }
func (t Target) GetName() string      { return t.Name }
func (t *Target) SetFields(d map[string]string) {
	t.Name = d["name"]
	t.Description = d["description"]
}

// --- Tool ---

func (t Tool) GetID() bson.ObjectID { return t.ID }
func (t Tool) GetName() string      { return t.Name }
func (t *Tool) SetFields(d map[string]string) {
	t.Name = d["name"]
	t.Description = d["description"]
}

// --- Control ---

func (c Control) GetID() bson.ObjectID { return c.ID }
func (c Control) GetName() string      { return c.Name }
func (c *Control) SetFields(d map[string]string) {
	c.Name = d["name"]
	c.Description = d["description"]
}

// --- Tag ---

func (t Tag) GetID() bson.ObjectID { return t.ID }
func (t Tag) GetName() string      { return t.Name }
func (t *Tag) SetFields(d map[string]string) {
	t.Name = d["name"]
	t.Colour = d["colour"]
}

// --- Datasource ---

func (d Datasource) GetID() bson.ObjectID { return d.ID }
func (d Datasource) GetName() string      { return d.Name }
func (d *Datasource) SetFields(data map[string]string) {
	d.Name = data["name"]
	d.Description = data["description"]
}

// --- DetectionRule ---

func (r DetectionRule) GetID() bson.ObjectID { return r.ID }
func (r DetectionRule) GetName() string      { return r.Name }
func (r *DetectionRule) SetFields(d map[string]string) {
	r.Name = d["name"]
	r.Description = d["description"]
}
