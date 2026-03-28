package handler

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"time"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// samlSP is the package-level SAML service provider, initialised at startup.
var samlSP *samlsp.Middleware

// samlACSURL is the expected ACS URL for recipient validation.
var samlACSURL string

// InitSAML sets up the SAML service provider from the application config.
// Call this at startup if SAML is enabled.
func InitSAML(cfg *config.Config) error {
	rootURL, err := url.Parse(cfg.SAMLRootURL)
	if err != nil {
		return err
	}

	// Load SP certificate and key.
	keyPair, err := tls.LoadX509KeyPair(cfg.SAMLCertFile, cfg.SAMLKeyFile)
	if err != nil {
		return err
	}

	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return err
	}

	// Fetch IdP metadata.
	idpMetadataURL, err := url.Parse(cfg.SAMLIDPMetadataURL)
	if err != nil {
		return err
	}

	idpMetadata, err := samlsp.FetchMetadata(
		context.Background(),
		http.DefaultClient,
		*idpMetadataURL,
	)
	if err != nil {
		return err
	}

	opts := samlsp.Options{
		URL:         *rootURL,
		Key:         keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate: keyPair.Leaf,
		IDPMetadata: idpMetadata,
	}

	if cfg.SAMLEntityID != "" {
		entityIDURL, err := url.Parse(cfg.SAMLEntityID)
		if err == nil {
			opts.EntityID = entityIDURL.String()
		}
	}

	samlSP, err = samlsp.New(opts)
	if err != nil {
		return err
	}

	// Store the ACS URL for recipient validation during assertion parsing.
	samlACSURL = cfg.SAMLRootURL + "/auth/saml/acs"

	log.Printf("SAML SP initialized: acs=%s", samlACSURL)
	return nil
}

// HandleSAMLMetadata serves the SP metadata XML.
func HandleSAMLMetadata(c *gin.Context) {
	if samlSP == nil {
		c.String(http.StatusNotFound, "SAML not configured")
		return
	}
	samlSP.ServeMetadata(c.Writer, c.Request)
}

// HandleSAMLLogin initiates SAML SP-initiated SSO by redirecting to the IdP.
func HandleSAMLLogin(c *gin.Context) {
	if samlSP == nil {
		c.String(http.StatusNotFound, "SAML not configured")
		return
	}
	// The middleware's RequireAccount will trigger a redirect to the IdP.
	samlSP.HandleStartAuthFlow(c.Writer, c.Request)
}

// HandleSAMLACS processes the SAML assertion from the IdP.
func HandleSAMLACS(c *gin.Context) {
	if samlSP == nil {
		c.String(http.StatusNotFound, "SAML not configured")
		return
	}

	sess := auth.GetSession(c.Request)
	setFlash := func(msg string) {
		sess.Values["flash"] = msg
		sess.Values["flash_category"] = "danger"
		if err := auth.SaveSession(c.Writer, c.Request, sess); err != nil {
			c.String(http.StatusInternalServerError, "Internal error")
			return
		}
		c.Redirect(http.StatusFound, "/login")
	}

	err := c.Request.ParseForm()
	if err != nil {
		setFlash("SAML login failed: invalid response.")
		return
	}

	assertion, err := samlSP.ServiceProvider.ParseResponse(c.Request, []string{samlACSURL})
	if err != nil {
		log.Printf("SAML assertion parse error: %v", err)
		setFlash("SAML login failed: could not validate assertion.")
		return
	}

	// Extract email from SAML attributes.
	email := getSAMLAttribute(assertion, "email",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		"urn:oid:0.9.2342.19200300.100.1.3", // mail
		"http://schemas.xmlsoap.org/claims/EmailAddress",
	)
	if email == "" {
		// Try NameID as fallback.
		if assertion.Subject != nil && assertion.Subject.NameID != nil {
			email = assertion.Subject.NameID.Value
		}
	}

	if email == "" {
		setFlash("SAML login failed: no email address in assertion.")
		return
	}

	// Validate email format.
	if _, err := mail.ParseAddress(email); err != nil {
		setFlash("SAML login failed: invalid email format in assertion.")
		return
	}

	// Extract display name.
	username := getSAMLAttribute(assertion, "displayName",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		"urn:oid:2.16.840.1.113730.3.1.241", // displayName
		"http://schemas.xmlsoap.org/claims/CommonName",
	)

	// Find or create user.
	cfg := config.Cfg
	user, err := models.FindOrCreateSSOUser(c.Request.Context(), email, username, "saml", cfg.SSODefaultRole, cfg.SSOAutoProvision)
	if err != nil {
		log.Printf("SAML user provisioning failed: %v", err)
		setFlash("SAML login failed: internal error.")
		return
	}
	if user == nil {
		setFlash("No account found for " + email + ". Contact an administrator.")
		return
	}

	if !user.Active {
		setFlash("Account is disabled.")
		return
	}

	// Update login tracking.
	now := time.Now().UTC()
	db.Col(db.ColUser).UpdateOne(c.Request.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"last_login_at":    user.CurrentLoginAt,
			"last_login_ip":    user.CurrentLoginIP,
			"current_login_at": &now,
			"current_login_ip": c.Request.RemoteAddr,
		},
		"$inc": bson.M{"login_count": 1},
	})

	auth.SetSessionUser(c.Writer, c.Request, user.ID.Hex())
	c.Redirect(http.StatusFound, "/")
}

// getSAMLAttribute searches for a value across multiple attribute name variants.
func getSAMLAttribute(assertion *saml.Assertion, names ...string) string {
	for _, stmt := range assertion.AttributeStatements {
		for _, attr := range stmt.Attributes {
			for _, name := range names {
				if attr.Name == name || attr.FriendlyName == name {
					if len(attr.Values) > 0 {
						return attr.Values[0].Value
					}
				}
			}
		}
	}
	return ""
}
