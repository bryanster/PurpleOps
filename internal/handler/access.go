package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/bryanster/purpleops/internal/render"
	"github.com/flosch/pongo2/v6"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// UserDisplay wraps User with pre-computed template-friendly fields.
type UserDisplay struct {
	models.User
	RoleNames       string
	AssessmentNames string
}

// HandleAccessPage renders the user management page.
// GET /manage/access
func HandleAccessPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Load all users.
	var users []models.User
	cursor, err := db.Col(db.ColUser).Find(ctx, bson.M{})
	if err == nil {
		cursor.All(ctx, &users)
	}

	// Load all assessments.
	var assessments []models.Assessment
	cursor, err = db.Col(db.ColAssessment).Find(ctx, bson.M{})
	if err == nil {
		cursor.All(ctx, &assessments)
	}

	// Load all roles.
	var roles []models.Role
	cursor, err = db.Col(db.ColRole).Find(ctx, bson.M{})
	if err == nil {
		cursor.All(ctx, &roles)
	}

	// Build display wrappers with pre-computed role/assessment names.
	displayUsers := make([]UserDisplay, len(users))
	for i, u := range users {
		displayUsers[i] = UserDisplay{
			User:            u,
			RoleNames:       strings.Join(u.GetRoleNames(ctx), ", "),
			AssessmentNames: strings.Join(u.GetAssessmentNames(ctx), ", "),
		}
	}

	render.Render(w, r, "access.html", pongo2.Context{
		"users":       displayUsers,
		"assessments": assessments,
		"roles":       roles,
	})
}

// HandleCreateUser creates a new user.
// POST /manage/access/user
func HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	username := r.FormValue("username")
	password := r.FormValue("password")

	if email == "" || username == "" || password == "" {
		http.Error(w, "Email, username, and password are required", http.StatusBadRequest)
		return
	}

	// Hash the password.
	hashed, err := auth.HashPassword(password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// Resolve role names to IDs.
	roleNames := r.Form["roles"]
	var roleIDs []bson.ObjectID
	for _, name := range roleNames {
		role, err := models.FindRole(ctx, name)
		if err == nil {
			roleIDs = append(roleIDs, role.ID)
		}
	}

	// Resolve assessment names to IDs.
	assessmentNames := r.Form["assessments"]
	var assessmentIDs []bson.ObjectID
	for _, name := range assessmentNames {
		var a models.Assessment
		err := db.Col(db.ColAssessment).FindOne(ctx, bson.M{"name": name}).Decode(&a)
		if err == nil {
			assessmentIDs = append(assessmentIDs, a.ID)
		}
	}

	user := models.User{
		ID:          bson.NewObjectID(),
		Email:       email,
		Username:    username,
		Password:    hashed,
		Roles:       roleIDs,
		Assessments: assessmentIDs,
		Active:      true,
		InitPwd:     true,
	}

	if _, err := db.Col(db.ColUser).InsertOne(ctx, &user); err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.ToJSON(ctx, false))
}

// HandleEditUser updates an existing user.
// POST /manage/access/user/{id}
func HandleEditUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	user, err := models.FindUser(ctx, id)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Can't rename the built-in admin.
	newUsername := r.FormValue("username")
	if user.Username == "admin" && newUsername != "admin" {
		http.Error(w, "Cannot rename the built-in admin user", http.StatusBadRequest)
		return
	}

	updates := bson.M{}

	if email := r.FormValue("email"); email != "" {
		updates["email"] = email
	}
	if newUsername != "" {
		updates["username"] = newUsername
	}

	// Update password if provided and not blank.
	// Note: editUserModal fills password with spaces to indicate "unchanged" — trim to detect this.
	if password := strings.TrimSpace(r.FormValue("password")); password != "" {
		hashed, err := auth.HashPassword(password)
		if err != nil {
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}
		updates["password"] = hashed
	}

	// Resolve role names to IDs.
	roleNames := r.Form["roles"]
	var roleIDs []bson.ObjectID
	isAdmin := false
	for _, name := range roleNames {
		if name == "Admin" {
			isAdmin = true
		}
		role, err := models.FindRole(ctx, name)
		if err == nil {
			roleIDs = append(roleIDs, role.ID)
		}
	}

	// Can't de-admin the built-in admin.
	if user.Username == "admin" && !isAdmin {
		http.Error(w, "Cannot remove Admin role from the built-in admin user", http.StatusBadRequest)
		return
	}

	updates["roles"] = roleIDs

	// Resolve assessment names to IDs.
	// Admin users get assessments cleared (they have implied access to all).
	if isAdmin {
		updates["assessments"] = []bson.ObjectID{}
	} else {
		assessmentNames := r.Form["assessments"]
		var assessmentIDs []bson.ObjectID
		for _, name := range assessmentNames {
			var a models.Assessment
			err := db.Col(db.ColAssessment).FindOne(ctx, bson.M{"name": name}).Decode(&a)
			if err == nil {
				assessmentIDs = append(assessmentIDs, a.ID)
			}
		}
		updates["assessments"] = assessmentIDs
	}

	_, err = db.Col(db.ColUser).UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": updates})
	if err != nil {
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	// Reload user to return updated data.
	user, _ = models.FindUser(ctx, id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.ToJSON(ctx, false))
}

// HandleDeleteUser deletes a user.
// DELETE /manage/access/user/{id}
func HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	user, err := models.FindUser(ctx, id)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Can't delete the built-in admin.
	if user.Username == "admin" {
		http.Error(w, "Cannot delete the built-in admin user", http.StatusBadRequest)
		return
	}

	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	_, err = db.Col(db.ColUser).DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
