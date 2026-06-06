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
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// FlashMessage represents a flash message for templates.
type FlashMessage struct {
	Category string
	Message  string
}

// getFlashMessages reads flash messages from the session and clears them.
func getFlashMessages(c *gin.Context) []FlashMessage {
	sess := auth.GetSession(c)
	if flash, ok := sess.Get("flash").(string); ok && flash != "" {
		category, _ := sess.Get("flash_category").(string)
		sess.Delete("flash")
		sess.Delete("flash_category")
		if err := auth.SaveSession(c, sess); err != nil {
			return nil
		}
		return []FlashMessage{{Category: category, Message: flash}}
	}
	return nil
}

// HandleLogin renders the login page.
func HandleLogin(c *gin.Context) {
	ctx := pongo2.Context{
		"oauth_enabled":       config.Cfg.OAuthEnabled,
		"oauth_provider_name": config.Cfg.OAuthProviderName,
		"saml_enabled":        config.Cfg.SAMLEnabled,
	}
	if msgs := getFlashMessages(c); msgs != nil {
		ctx["flash_messages"] = msgs
	}
	render.Render(c.Writer, c.Request, "login.html", ctx)
}

// HandleLoginPost authenticates a user with email and password.
func HandleLoginPost(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "Bad request")
		return
	}

	email := strings.TrimSpace(c.Request.FormValue("email"))
	password := c.Request.FormValue("password")

	sess := auth.GetSession(c)

	setFlash := func(msg, category string) {
		sess.Set("flash", msg)
		sess.Set("flash_category", category)
		if err := auth.SaveSession(c, sess); err != nil {
			c.String(http.StatusInternalServerError, "Internal error")
			return
		}
		c.Redirect(http.StatusFound, "/login")
	}

	if email == "" || password == "" {
		setFlash("Please provide email and password.", "danger")
		return
	}

	user, err := models.FindUserByEmail(c.Request.Context(), email)
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
	db.Col(db.ColUser).UpdateOne(c.Request.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"last_login_at":    user.CurrentLoginAt,
			"last_login_ip":    user.CurrentLoginIP,
			"current_login_at": &now,
			"current_login_ip": c.ClientIP(),
		},
		"$inc": bson.M{
			"login_count": 1,
		},
	})

	// If MFA is enabled globally and the user has a TOTP secret, redirect to verify.
	if config.Cfg.MFA && user.TFSecret != "" {
		sess.Set("mfa_user_id", user.ID.Hex())
		if err := auth.SaveSession(c, sess); err != nil {
			c.String(http.StatusInternalServerError, "Internal error")
			return
		}
		c.Redirect(http.StatusFound, "/mfa/verify")
		return
	}

	auth.SetSessionUser(c, user.ID.Hex())
	c.Redirect(http.StatusFound, "/")
}

// HandleLogout clears the session and redirects to login.
func HandleLogout(c *gin.Context) {
	auth.ClearSession(c)
	c.Redirect(http.StatusFound, "/login")
}

// HandlePasswordChange renders the password change form.
func HandlePasswordChange(c *gin.Context) {
	ctx := pongo2.Context{}
	if msgs := getFlashMessages(c); msgs != nil {
		ctx["flash_messages"] = msgs
	}
	render.Render(c.Writer, c.Request, "password_change.html", ctx)
}

// HandlePasswordChangePost validates and updates the user's password.
func HandlePasswordChangePost(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "Bad request")
		return
	}

	user := auth.UserFromContext(c.Request.Context())
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	currentPassword := c.Request.FormValue("password")
	newPassword := c.Request.FormValue("new_password")
	confirmPassword := c.Request.FormValue("new_password_confirm")

	sess := auth.GetSession(c)

	setFlash := func(msg, category string) {
		sess.Set("flash", msg)
		sess.Set("flash_category", category)
		if err := auth.SaveSession(c, sess); err != nil {
			c.String(http.StatusInternalServerError, "Internal error")
			return
		}
		c.Redirect(http.StatusFound, "/password/change")
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

	_, err = db.Col(db.ColUser).UpdateOne(c.Request.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"password": hashed},
	})
	if err != nil {
		setFlash("Failed to update password.", "danger")
		return
	}

	c.Redirect(http.StatusFound, "/password/changed")
}

// HandlePasswordChanged marks the user's initial password as changed and redirects home.
func HandlePasswordChanged(c *gin.Context) {
	user := auth.UserFromContext(c.Request.Context())
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	db.Col(db.ColUser).UpdateOne(c.Request.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"initpwd": false},
	})

	c.Redirect(http.StatusFound, "/")
}

// HandleMFARegister renders the MFA registration page.
func HandleMFARegister(c *gin.Context) {
	render.Render(c.Writer, c.Request, "mfa_register.html", nil)
}

// HandleMFARegisterPost generates a TOTP key for the user and returns the provisioning URI.
func HandleMFARegisterPost(c *gin.Context) {
	user := auth.UserFromContext(c.Request.Context())
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      config.Cfg.Name,
		AccountName: user.Email,
	})
	if err != nil {
		sess := auth.GetSession(c)
		sess.Set("flash", "Failed to generate MFA key.")
		sess.Set("flash_category", "danger")
		if err := auth.SaveSession(c, sess); err != nil {
			c.String(http.StatusInternalServerError, "Internal error")
			return
		}
		c.Redirect(http.StatusFound, "/mfa/register")
		return
	}

	// Store the secret on the user record.
	db.Col(db.ColUser).UpdateOne(c.Request.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"tf_totp_secret":    key.Secret(),
			"tf_primary_method": "totp",
		},
	})

	render.Render(c.Writer, c.Request, "mfa_register.html", pongo2.Context{
		"qr_url":      key.URL(),
		"totp_secret": key.Secret(),
	})
}

// HandleMFAVerify renders the MFA verification page.
func HandleMFAVerify(c *gin.Context) {
	render.Render(c.Writer, c.Request, "mfa_verify.html", nil)
}

// HandleMFAVerifyPost validates the TOTP code and completes authentication.
func HandleMFAVerifyPost(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "Bad request")
		return
	}

	sess := auth.GetSession(c)
	code := strings.TrimSpace(c.Request.FormValue("code"))

	setFlash := func(msg, category string) {
		sess.Set("flash", msg)
		sess.Set("flash_category", category)
		if err := auth.SaveSession(c, sess); err != nil {
			c.String(http.StatusInternalServerError, "Internal error")
			return
		}
		c.Redirect(http.StatusFound, "/mfa/verify")
	}

	// Get the user ID stored during login.
	userID, ok := sess.Get("mfa_user_id").(string)
	if !ok || userID == "" {
		// Fall back to authenticated user (for post-login MFA setup verification).
		user := auth.UserFromContext(c.Request.Context())
		if user == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}
		userID = user.ID.Hex()
	}

	user, err := models.FindUser(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.Redirect(http.StatusFound, "/login")
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
	sess.Delete("mfa_user_id")
	if err := auth.SaveSession(c, sess); err != nil {
		c.String(http.StatusInternalServerError, "Internal error")
		return
	}

	auth.SetSessionUser(c, user.ID.Hex())
	c.Redirect(http.StatusFound, "/")
}
