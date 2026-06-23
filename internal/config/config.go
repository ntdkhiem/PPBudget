package config

import (
	"log"
	"os"
)

type Config struct {
	DatabaseURL   string
	Port          string
	IngestAPIKey  string
	JWTSecret     string
	AdminPassword string
}

func Load() *Config {
	cfg := &Config{
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://firefly:firefly_password@localhost:5432/firefly?sslmode=disable"),
		Port:          getEnv("PORT", "8080"),
		IngestAPIKey:  getEnv("INGEST_API_KEY", "super-secret-api-key"),
		JWTSecret:     getEnv("JWT_SECRET", "super-secret-jwt-key"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "adminpassword"),
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
