package main

import (
	"os"
	"testing"
)

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
	os.Setenv("FLASK_DEBUG", "True")
	os.Setenv("FLASK_MFA", "True")
	defer func() {
		os.Unsetenv("MONGO_HOST")
		os.Unsetenv("MONGO_PORT")
		os.Unsetenv("FLASK_DEBUG")
		os.Unsetenv("FLASK_MFA")
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
