package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	MongoDB   string
	MongoHost string
	MongoPort int

	Debug      bool
	MFA        bool
	Host       string
	Port       string
	Name       string
	SecretKey  string
	PassSalt   string
	TOTPSecret string
	AdminPwd   string

	// OAuth2 SSO configuration.
	OAuthEnabled      bool
	OAuthProviderName string // Display name, e.g. "Google", "GitHub", "Azure AD"
	OAuthClientID     string
	OAuthClientSecret string
	OAuthAuthURL      string // Authorization endpoint
	OAuthTokenURL     string // Token endpoint
	OAuthUserInfoURL  string // UserInfo endpoint (must return JSON with "email" field)
	OAuthScopes       string // Comma-separated scopes (default: "openid,email,profile")
	OAuthRedirectURL  string // Full callback URL, e.g. "https://purpleops.example.com/auth/oauth/callback"

	// SAML SSO configuration.
	SAMLEnabled        bool
	SAMLIDPMetadataURL string // IdP metadata URL
	SAMLEntityID       string // SP entity ID (default: derived from root URL)
	SAMLRootURL        string // Root URL of this application, e.g. "https://purpleops.example.com"
	SAMLCertFile       string // Path to SP certificate file (PEM)
	SAMLKeyFile        string // Path to SP private key file (PEM)

	// SSO shared settings.
	SSODefaultRole   string // Default role for auto-provisioned SSO users (default: "Spectator")
	SSOAutoProvision bool   // Auto-create users on first SSO login (default: true)
}

// Cfg is the package-level config instance set by LoadConfig.
var Cfg *Config

// LoadConfig reads configuration from the environment (and .env file) and
// stores it in Cfg, returning the populated *Config.
func LoadConfig() *Config {
	_ = godotenv.Load()

	port, _ := strconv.Atoi(getEnv("MONGO_PORT", "27017"))
	ssoAutoProvision := getEnv("SSO_AUTO_PROVISION", "true")

	cfg := &Config{
		MongoDB:    getEnv("MONGO_DB", "assessments3"),
		MongoHost:  getEnv("MONGO_HOST", "localhost"),
		MongoPort:  port,
		Debug:      getEnv("DEBUG", "False") == "True",
		MFA:        getEnv("MFA", "False") == "True",
		Host:       getEnv("HOST", "0.0.0.0"),
		Port:       getEnv("PORT", "8888"),
		Name:       getEnv("NAME", "dev"),
		SecretKey:  getEnv("SECRET_KEY", "change-me"),
		PassSalt:   getEnv("PASSWORD_SALT", "change-me"),
		TOTPSecret: getEnv("TOTP_SECRET", ""),
		AdminPwd:   getEnv("POPS_ADMIN_PWD", ""),

		OAuthEnabled:      getEnv("OAUTH_ENABLED", "") == "true",
		OAuthProviderName: getEnv("OAUTH_PROVIDER_NAME", "SSO"),
		OAuthClientID:     getEnv("OAUTH_CLIENT_ID", ""),
		OAuthClientSecret: getEnv("OAUTH_CLIENT_SECRET", ""),
		OAuthAuthURL:      getEnv("OAUTH_AUTH_URL", ""),
		OAuthTokenURL:     getEnv("OAUTH_TOKEN_URL", ""),
		OAuthUserInfoURL:  getEnv("OAUTH_USERINFO_URL", ""),
		OAuthScopes:       getEnv("OAUTH_SCOPES", "openid,email,profile"),
		OAuthRedirectURL:  getEnv("OAUTH_REDIRECT_URL", ""),

		SAMLEnabled:        getEnv("SAML_ENABLED", "") == "true",
		SAMLIDPMetadataURL: getEnv("SAML_IDP_METADATA_URL", ""),
		SAMLEntityID:       getEnv("SAML_ENTITY_ID", ""),
		SAMLRootURL:        getEnv("SAML_ROOT_URL", ""),
		SAMLCertFile:       getEnv("SAML_CERT_FILE", ""),
		SAMLKeyFile:        getEnv("SAML_KEY_FILE", ""),

		SSODefaultRole:   getEnv("SSO_DEFAULT_ROLE", "Spectator"),
		SSOAutoProvision: ssoAutoProvision == "true" || ssoAutoProvision == "True",
	}
	if cfg.SecretKey == "change-me" {
		log.Println("WARNING: SECRET_KEY is set to the insecure default. Set this to a random value before deploying.")
	}
	if cfg.PassSalt == "change-me" {
		log.Println("WARNING: PASSWORD_SALT is set to the insecure default. Set this to a random value before deploying.")
	}

	Cfg = cfg
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
