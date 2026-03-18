package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
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
	ID                bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	Name              string          `bson:"name" json:"name"`
	Description       string          `bson:"description,omitempty" json:"description"`
	Created           *time.Time      `bson:"created,omitempty" json:"created"`
	Targets           []Target        `bson:"targets,omitempty" json:"targets"`
	Sources           []Source        `bson:"sources,omitempty" json:"sources"`
	Tools             []Tool          `bson:"tools,omitempty" json:"tools"`
	Controls          []Control       `bson:"controls,omitempty" json:"controls"`
	Tags              []Tag           `bson:"tags,omitempty" json:"tags"`
	Datasources       []Datasource    `bson:"datasources,omitempty" json:"datasources"`
	Rules             []DetectionRule `bson:"rules,omitempty" json:"rules"`
	DetectionSources  []Datasource    `bson:"detectionsources,omitempty" json:"detectionsources"`
	PreventionSources []Datasource    `bson:"preventionsources,omitempty" json:"preventionsources"`
	NavigatorExport   string          `bson:"navigatorexport,omitempty" json:"navigatorexport"`
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
	AuthProvider   string          `bson:"auth_provider,omitempty" json:"auth_provider,omitempty"` // "local", "oauth", "saml"
}

// APIKey represents a user-generated API key scoped to a subset of user permissions.
type APIKey struct {
	ID          bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	UserID      bson.ObjectID   `bson:"user_id" json:"user_id"`
	Name        string          `bson:"name" json:"name"`
	KeyHash     string          `bson:"key_hash" json:"-"`
	Prefix      string          `bson:"prefix" json:"prefix"`
	Roles       []bson.ObjectID `bson:"roles,omitempty" json:"-"`
	Assessments []bson.ObjectID `bson:"assessments,omitempty" json:"-"`
	CreatedAt   time.Time       `bson:"created_at" json:"created_at"`
	LastUsedAt  *time.Time      `bson:"last_used_at,omitempty" json:"last_used_at"`
	Active      bool            `bson:"active" json:"active"`
}
