package config

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Ensure tests don't trigger log.Fatal on default secrets.
	os.Setenv("SECRET_KEY", "test-secret-key")
	os.Setenv("PASSWORD_SALT", "test-password-salt")
	os.Exit(m.Run())
}

func TestGetEnv(t *testing.T) {
	// Test with unset variable
	os.Unsetenv("TEST_GETENV_VAR")
	if got := getEnv("TEST_GETENV_VAR", "default"); got != "default" {
		t.Errorf("getEnv with unset var = %q, want %q", got, "default")
	}

	// Test with set variable
	os.Setenv("TEST_GETENV_VAR", "custom")
	defer os.Unsetenv("TEST_GETENV_VAR")
	if got := getEnv("TEST_GETENV_VAR", "default"); got != "custom" {
		t.Errorf("getEnv with set var = %q, want %q", got, "custom")
	}

	// Test with empty string set
	os.Setenv("TEST_GETENV_EMPTY", "")
	defer os.Unsetenv("TEST_GETENV_EMPTY")
	if got := getEnv("TEST_GETENV_EMPTY", "fallback"); got != "fallback" {
		t.Errorf("getEnv with empty var = %q, want %q", got, "fallback")
	}
}

func TestLoadConfig(t *testing.T) {
	// LoadConfig calls godotenv.Load() which reads .env if present,
	// so we test with explicit env vars set to override .env values.

	os.Setenv("MONGO_HOST", "db.example.com")
	os.Setenv("MONGO_PORT", "27018")
	os.Setenv("DEBUG", "True")
	os.Setenv("MFA", "True")
	defer func() {
		os.Unsetenv("MONGO_HOST")
		os.Unsetenv("MONGO_PORT")
		os.Unsetenv("DEBUG")
		os.Unsetenv("MFA")
	}()

	cfg := LoadConfig()
	if cfg.MongoHost != "db.example.com" {
		t.Errorf("MongoHost = %q, want %q", cfg.MongoHost, "db.example.com")
	}
	if cfg.MongoPort != 27018 {
		t.Errorf("MongoPort = %d, want %d", cfg.MongoPort, 27018)
	}
	if cfg.Debug != true {
		t.Errorf("Debug = %v, want true", cfg.Debug)
	}
	if cfg.MFA != true {
		t.Errorf("MFA = %v, want true", cfg.MFA)
	}
	if cfg.MongoDB != "assessments3" {
		t.Errorf("MongoDB = %q, want %q", cfg.MongoDB, "assessments3")
	}
}

func TestLoadConfigInvalidPort(t *testing.T) {
	os.Setenv("MONGO_PORT", "invalid")
	defer os.Unsetenv("MONGO_PORT")

	cfg := LoadConfig()
	// strconv.Atoi returns 0 on error
	if cfg.MongoPort != 0 {
		t.Errorf("MongoPort with invalid value = %d, want 0", cfg.MongoPort)
	}
}

func TestLoadConfigOAuthEnabled(t *testing.T) {
	os.Setenv("OAUTH_ENABLED", "true")
	os.Setenv("OAUTH_PROVIDER_NAME", "Google")
	os.Setenv("OAUTH_CLIENT_ID", "google-client-id")
	os.Setenv("OAUTH_CLIENT_SECRET", "google-secret")
	os.Setenv("OAUTH_AUTH_URL", "https://accounts.google.com/o/oauth2/auth")
	os.Setenv("OAUTH_TOKEN_URL", "https://oauth2.googleapis.com/token")
	os.Setenv("OAUTH_USERINFO_URL", "https://openidconnect.googleapis.com/v1/userinfo")
	os.Setenv("OAUTH_SCOPES", "openid,email")
	os.Setenv("OAUTH_REDIRECT_URL", "https://purpleops.example.com/auth/oauth/callback")
	defer func() {
		for _, k := range []string{
			"OAUTH_ENABLED", "OAUTH_PROVIDER_NAME", "OAUTH_CLIENT_ID",
			"OAUTH_CLIENT_SECRET", "OAUTH_AUTH_URL", "OAUTH_TOKEN_URL",
			"OAUTH_USERINFO_URL", "OAUTH_SCOPES", "OAUTH_REDIRECT_URL",
		} {
			os.Unsetenv(k)
		}
	}()

	cfg := LoadConfig()
	if !cfg.OAuthEnabled {
		t.Error("expected OAuthEnabled=true")
	}
	if cfg.OAuthProviderName != "Google" {
		t.Errorf("expected OAuthProviderName 'Google', got %q", cfg.OAuthProviderName)
	}
	if cfg.OAuthClientID != "google-client-id" {
		t.Errorf("expected OAuthClientID 'google-client-id', got %q", cfg.OAuthClientID)
	}
	if cfg.OAuthScopes != "openid,email" {
		t.Errorf("expected OAuthScopes 'openid,email', got %q", cfg.OAuthScopes)
	}
}

func TestLoadConfigOAuthDisabledByDefault(t *testing.T) {
	os.Unsetenv("OAUTH_ENABLED")
	cfg := LoadConfig()
	if cfg.OAuthEnabled {
		t.Error("expected OAuthEnabled=false by default")
	}
	if cfg.OAuthProviderName != "SSO" {
		t.Errorf("expected default OAuthProviderName 'SSO', got %q", cfg.OAuthProviderName)
	}
}

func TestLoadConfigSAMLEnabled(t *testing.T) {
	os.Setenv("SAML_ENABLED", "true")
	os.Setenv("SAML_IDP_METADATA_URL", "https://idp.example.com/metadata")
	os.Setenv("SAML_ENTITY_ID", "https://purpleops.example.com")
	os.Setenv("SAML_ROOT_URL", "https://purpleops.example.com")
	os.Setenv("SAML_CERT_FILE", "/etc/saml/cert.pem")
	os.Setenv("SAML_KEY_FILE", "/etc/saml/key.pem")
	defer func() {
		for _, k := range []string{
			"SAML_ENABLED", "SAML_IDP_METADATA_URL", "SAML_ENTITY_ID",
			"SAML_ROOT_URL", "SAML_CERT_FILE", "SAML_KEY_FILE",
		} {
			os.Unsetenv(k)
		}
	}()

	cfg := LoadConfig()
	if !cfg.SAMLEnabled {
		t.Error("expected SAMLEnabled=true")
	}
	if cfg.SAMLIDPMetadataURL != "https://idp.example.com/metadata" {
		t.Errorf("expected SAMLIDPMetadataURL, got %q", cfg.SAMLIDPMetadataURL)
	}
	if cfg.SAMLCertFile != "/etc/saml/cert.pem" {
		t.Errorf("expected SAMLCertFile '/etc/saml/cert.pem', got %q", cfg.SAMLCertFile)
	}
}

func TestLoadConfigSAMLDisabledByDefault(t *testing.T) {
	os.Unsetenv("SAML_ENABLED")
	cfg := LoadConfig()
	if cfg.SAMLEnabled {
		t.Error("expected SAMLEnabled=false by default")
	}
}

func TestLoadConfigSSODefaults(t *testing.T) {
	os.Unsetenv("SSO_DEFAULT_ROLE")
	os.Unsetenv("SSO_AUTO_PROVISION")
	cfg := LoadConfig()

	if cfg.SSODefaultRole != "Spectator" {
		t.Errorf("expected SSODefaultRole 'Spectator', got %q", cfg.SSODefaultRole)
	}
	if !cfg.SSOAutoProvision {
		t.Error("expected SSOAutoProvision=true by default")
	}
}

func TestLoadConfigSSOAutoProvisionFalse(t *testing.T) {
	os.Setenv("SSO_AUTO_PROVISION", "false")
	defer os.Unsetenv("SSO_AUTO_PROVISION")

	cfg := LoadConfig()
	if cfg.SSOAutoProvision {
		t.Error("expected SSOAutoProvision=false when set to 'false'")
	}
}

func TestLoadConfigSSODefaultRoleCustom(t *testing.T) {
	os.Setenv("SSO_DEFAULT_ROLE", "Blue")
	defer os.Unsetenv("SSO_DEFAULT_ROLE")

	cfg := LoadConfig()
	if cfg.SSODefaultRole != "Blue" {
		t.Errorf("expected SSODefaultRole 'Blue', got %q", cfg.SSODefaultRole)
	}
}
