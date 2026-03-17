package main

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

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
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	port, _ := strconv.Atoi(getEnv("MONGO_PORT", "27017"))
	return &Config{
		MongoDB:    getEnv("MONGO_DB", "assessments3"),
		MongoHost:  getEnv("MONGO_HOST", "localhost"),
		MongoPort:  port,
		Debug:      getEnv("FLASK_DEBUG", "False") == "True",
		MFA:        getEnv("FLASK_MFA", "False") == "True",
		Host:       getEnv("HOST", "0.0.0.0"),
		Port:       getEnv("PORT", "8888"),
		Name:       getEnv("NAME", "dev"),
		SecretKey:  getEnv("FLASK_SECRET_KEY", "change-me"),
		PassSalt:   getEnv("FLASK_SECURITY_PASSWORD_SALT", "change-me"),
		TOTPSecret: getEnv("FLASK_SECURITY_TOTP_SECRETS", ""),
		AdminPwd:   getEnv("POPS_ADMIN_PWD", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
