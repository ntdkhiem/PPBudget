package config

import (
	"log"
	"os"
)

type Config struct {
	DatabaseURL     string
	Port            string
	IngestAPIKey    string
	JWTSecret       string
	AdminPassword   string
	SMTPHost        string
	SMTPPort        string
	SMTPUser        string
	SMTPPass        string
	ResendAPIKey    string
	ResendFromEmail string
	FrontendURL     string
}

func Load() *Config {
	cfg := &Config{
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://ppbudget:ppbudget_password@localhost:5432/ppbudget?sslmode=disable"),
		Port:            getEnv("PORT", "8080"),
		IngestAPIKey:    getEnv("INGEST_API_KEY", "super-secret-api-key"),
		JWTSecret:       getEnv("JWT_SECRET", "super-secret-jwt-key"),
		AdminPassword:   getEnv("ADMIN_PASSWORD", "adminpassword"),
		SMTPHost:        getEnv("SMTP_HOST", ""),
		SMTPPort:        getEnv("SMTP_PORT", "587"),
		SMTPUser:        getEnv("SMTP_USER", ""),
		SMTPPass:        getEnv("SMTP_PASS", ""),
		ResendAPIKey:    getEnv("RESEND_API_KEY", ""),
		ResendFromEmail: getEnv("RESEND_FROM_EMAIL", "onboarding@resend.dev"),
		FrontendURL:     getEnv("FRONTEND_URL", "http://localhost:3000"),
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return fallback
}

func (c *Config) MustLoad() {
	if c.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
}
