package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/bryanster/purpleops/internal/render"
	"github.com/flosch/pongo2/v6"
	"github.com/pquerna/otp/totp"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// FlashMessage represents a flash message for templates.
type FlashMessage struct {
	Category string
	Message  string
}

// getFlashMessages reads flash messages from the session and clears them.
func getFlashMessages(w http.ResponseWriter, r *http.Request) []FlashMessage {
	sess := auth.GetSession(r)
	if flash, ok := sess.Values["flash"].(string); ok && flash != "" {
		category, _ := sess.Values["flash_category"].(string)
		delete(sess.Values, "flash")
		delete(sess.Values, "flash_category")
		sess.Save(r, w)
		return []FlashMessage{{Category: category, Message: flash}}
	}
	return nil
}

// HandleLogin renders the login page.
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	ctx := pongo2.Context{
		"oauth_enabled":       config.Cfg.OAuthEnabled,
		"oauth_provider_name": config.Cfg.OAuthProviderName,
		"saml_enabled":        config.Cfg.SAMLEnabled,
	}
	if msgs := getFlashMessages(w, r); msgs != nil {
		ctx["flash_messages"] = msgs
	}
	render.Render(w, r, "login.html", ctx)
}

// HandleLoginPost authenticates a user with email and password.
func HandleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	sess := auth.GetSession(r)

	setFlash := func(msg, category string) {
		sess.Values["flash"] = msg
		sess.Values["flash_category"] = category
		sess.Save(r, w)
		http.Redirect(w, r, "/login", http.StatusFound)
	}

	if email == "" || password == "" {
		setFlash("Please provide email and password.", "danger")
		return
	}

	user, err := models.FindUserByEmail(r.Context(), email)
	if err != nil || user == nil {
		setFlash("Invalid email or password.", "danger")
		return
	}

	if !user.Active {
		setFlash("Account is disabled.", "danger")
		return
	}

	if !auth.CheckPassword(user.Password, password) {
		setFlash("Invalid email or password.", "danger")
		return
	}

	// Update login tracking fields.
	now := time.Now().UTC()
	db.Col(db.ColUser).UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"last_login_at":    user.CurrentLoginAt,
			"last_login_ip":    user.CurrentLoginIP,
			"current_login_at": &now,
			"current_login_ip": r.RemoteAddr,
		},
		"$inc": bson.M{
			"login_count": 1,
		},
	})

	// If MFA is enabled globally and the user has a TOTP secret, redirect to verify.
	if config.Cfg.MFA && user.TFSecret != "" {
		sess.Values["mfa_user_id"] = user.ID.Hex()
		sess.Save(r, w)
		http.Redirect(w, r, "/mfa/verify", http.StatusFound)
		return
	}

	auth.SetSessionUser(w, r, user.ID.Hex())
	http.Redirect(w, r, "/", http.StatusFound)
}

// HandleLogout clears the session and redirects to login.
func HandleLogout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSession(w, r)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// HandlePasswordChange renders the password change form.
func HandlePasswordChange(w http.ResponseWriter, r *http.Request) {
	ctx := pongo2.Context{}
	if msgs := getFlashMessages(w, r); msgs != nil {
		ctx["flash_messages"] = msgs
	}
	render.Render(w, r, "password_change.html", ctx)
}

// HandlePasswordChangePost validates and updates the user's password.
func HandlePasswordChangePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	currentPassword := r.FormValue("password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("new_password_confirm")

	sess := auth.GetSession(r)

	setFlash := func(msg, category string) {
		sess.Values["flash"] = msg
		sess.Values["flash_category"] = category
		sess.Save(r, w)
		http.Redirect(w, r, "/password/change", http.StatusFound)
	}

	// Verify current password.
	if !auth.CheckPassword(user.Password, currentPassword) {
		setFlash("Current password is incorrect.", "danger")
		return
	}

	// Validate new password length.
	if len(newPassword) < 12 {
		setFlash("New password must be at least 12 characters.", "danger")
		return
	}

	// Confirm passwords match.
	if newPassword != confirmPassword {
		setFlash("New passwords do not match.", "danger")
		return
	}

	hashed, err := auth.HashPassword(newPassword)
	if err != nil {
		setFlash("Internal error.", "danger")
		return
	}

	_, err = db.Col(db.ColUser).UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"password": hashed},
	})
	if err != nil {
		setFlash("Failed to update password.", "danger")
		return
	}

	http.Redirect(w, r, "/password/changed", http.StatusFound)
}

// HandlePasswordChanged marks the user's initial password as changed and redirects home.
func HandlePasswordChanged(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	db.Col(db.ColUser).UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"initpwd": false},
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// HandleMFARegister renders the MFA registration page.
func HandleMFARegister(w http.ResponseWriter, r *http.Request) {
	render.Render(w, r, "mfa_register.html", nil)
}

// HandleMFARegisterPost generates a TOTP key for the user and returns the provisioning URI.
func HandleMFARegisterPost(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      config.Cfg.Name,
		AccountName: user.Email,
	})
	if err != nil {
		sess := auth.GetSession(r)
		sess.Values["flash"] = "Failed to generate MFA key."
		sess.Values["flash_category"] = "danger"
		sess.Save(r, w)
		http.Redirect(w, r, "/mfa/register", http.StatusFound)
		return
	}

	// Store the secret on the user record.
	db.Col(db.ColUser).UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"tf_totp_secret":    key.Secret(),
			"tf_primary_method": "totp",
		},
	})

	render.Render(w, r, "mfa_register.html", pongo2.Context{
		"qr_url":      key.URL(),
		"totp_secret": key.Secret(),
	})
}

// HandleMFAVerify renders the MFA verification page.
func HandleMFAVerify(w http.ResponseWriter, r *http.Request) {
	render.Render(w, r, "mfa_verify.html", nil)
}

// HandleMFAVerifyPost validates the TOTP code and completes authentication.
func HandleMFAVerifyPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	sess := auth.GetSession(r)
	code := strings.TrimSpace(r.FormValue("code"))

	setFlash := func(msg, category string) {
		sess.Values["flash"] = msg
		sess.Values["flash_category"] = category
		sess.Save(r, w)
		http.Redirect(w, r, "/mfa/verify", http.StatusFound)
	}

	// Get the user ID stored during login.
	userID, ok := sess.Values["mfa_user_id"].(string)
	if !ok || userID == "" {
		// Fall back to authenticated user (for post-login MFA setup verification).
		user := auth.UserFromContext(r.Context())
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		userID = user.ID.Hex()
	}

	user, err := models.FindUser(r.Context(), userID)
	if err != nil || user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if code == "" {
		setFlash("Please enter your verification code.", "danger")
		return
	}

	valid := totp.Validate(code, user.TFSecret)
	if !valid {
		setFlash("Invalid verification code.", "danger")
		return
	}

	// Clear the temporary MFA user ID and set the full session.
	delete(sess.Values, "mfa_user_id")
	sess.Save(r, w)

	auth.SetSessionUser(w, r, user.ID.Hex())
	http.Redirect(w, r, "/", http.StatusFound)
}
