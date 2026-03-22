package render

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/flosch/pongo2/v6"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// templateSet is the package-level pongo2 template set.
var templateSet *pongo2.TemplateSet

// debugMode controls whether detailed errors are shown to clients.
var debugMode bool

// InitTemplates sets up the pongo2 template engine from the "templates" directory.
func InitTemplates(debug bool) {
	loader := pongo2.MustNewLocalFileSystemLoader("templates")
	templateSet = pongo2.NewSet("purpleops", loader)
	templateSet.Debug = debug
	debugMode = debug
}

// Render executes the named template and writes the result to w.
func Render(w http.ResponseWriter, r *http.Request, templateName string, ctx pongo2.Context) {
	if ctx == nil {
		ctx = pongo2.Context{}
	}

	// Always inject current_user and request into template context.
	user := auth.UserFromContext(r.Context())
	if user == nil {
		user = auth.GetCurrentUser(r)
	}
	ctx["current_user"] = &TemplateUser{user: user, ctx: r.Context()}
	ctx["request"] = &TemplateRequest{r: r}


	tpl, err := templateSet.FromFile(templateName)
	if err != nil {
		log.Printf("Template error: %v", err)
		if debugMode {
			http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	out, err := tpl.Execute(ctx)
	if err != nil {
		log.Printf("Template render error: %v", err)
		if debugMode {
			http.Error(w, fmt.Sprintf("Template render error: %v", err), http.StatusInternalServerError)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(out))
}

// TemplateUser wraps a User for template-friendly method access.
type TemplateUser struct {
	user *models.User
	ctx  context.Context
}

func (tu *TemplateUser) IsAuthenticated() bool {
	return tu.user != nil
}

func (tu *TemplateUser) HasRole(role string) bool {
	if tu.user == nil {
		return false
	}
	return tu.user.HasRole(tu.ctx, role)
}

func (tu *TemplateUser) GetInitpwd() bool {
	if tu.user == nil {
		return false
	}
	return tu.user.InitPwd
}

func (tu *TemplateUser) GetUsername() string {
	if tu.user == nil {
		return ""
	}
	return tu.user.Username
}

func (tu *TemplateUser) GetID() string {
	if tu.user == nil {
		return ""
	}
	return tu.user.ID.Hex()
}

func (tu *TemplateUser) AssessmentList() []string {
	if tu.user == nil {
		return nil
	}
	ids := tu.user.AssessmentList(tu.ctx)
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.Hex()
	}
	return strs
}

// TemplateRequest wraps http.Request for template-friendly access.
type TemplateRequest struct {
	r *http.Request
}

func (tr *TemplateRequest) GetPath() string {
	return tr.r.URL.Path
}

// NewTemplateUser constructs a TemplateUser — used in tests.
func NewTemplateUser(user *models.User, ctx context.Context) *TemplateUser {
	return &TemplateUser{user: user, ctx: ctx}
}

// NewTemplateRequest constructs a TemplateRequest — used in tests.
func NewTemplateRequest(r *http.Request) *TemplateRequest {
	return &TemplateRequest{r: r}
}

// ObjectIDHex is a helper that converts a bson.ObjectID to its hex string.
// Exposed so handler packages can use it without importing bson directly for templates.
func ObjectIDHex(id bson.ObjectID) string {
	return id.Hex()
}
