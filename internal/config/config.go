package config

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL     string
	Port            string
	IngestAPIKey    string
	JWTSecret       string
	SMTPHost        string
	SMTPPort        string
	SMTPUser        string
	SMTPPass        string
	ResendAPIKey    string
	ResendFromEmail string
	FrontendURL     string
}

// Placeholder values used when the corresponding environment variable is
// unset. They are committed to this repository, so a deployment running with
// them is running with publicly known secrets -- Validate refuses to start in
// that state outside local development.
const (
	defaultJWTSecret    = "super-secret-jwt-key"
	defaultIngestAPIKey = "super-secret-api-key"
	defaultFrontendURL  = "http://localhost:3000"
)

func Load() *Config {
	cfg := &Config{
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://ppbudget:ppbudget_password@localhost:5432/ppbudget?sslmode=disable"),
		Port:            getEnv("PORT", "8080"),
		IngestAPIKey:    getEnv("INGEST_API_KEY", defaultIngestAPIKey),
		JWTSecret:       getEnv("JWT_SECRET", defaultJWTSecret),
		SMTPHost:        getEnv("SMTP_HOST", ""),
		SMTPPort:        getEnv("SMTP_PORT", "587"),
		SMTPUser:        getEnv("SMTP_USER", ""),
		SMTPPass:        getEnv("SMTP_PASS", ""),
		ResendAPIKey:    getEnv("RESEND_API_KEY", ""),
		ResendFromEmail: getEnv("RESEND_FROM_EMAIL", "onboarding@resend.dev"),
		FrontendURL:     getEnv("FRONTEND_URL", defaultFrontendURL),
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return fallback
}

// MustLoad validates the configuration and exits on failure.
//
// Deprecated in favour of Validate, which returns the error so the caller can
// log it through its own handler. Kept as a thin wrapper for any caller that
// still wants fatal-on-invalid behaviour.
func (c *Config) MustLoad() {
	if err := c.Validate(); err != nil {
		log.Fatal(err)
	}
}

// Validate reports whether the configuration is safe to run with. The returned
// error is suitable for showing to an operator: it names the variables at fault
// and what to do about them.
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	// JWT_SECRET signs every session token and RequireJWT trusts the user_id
	// claim it carries, so running with the committed placeholder means anyone
	// who has seen this repository can mint a token for any account. The same
	// goes for INGEST_API_KEY, which guards the ingest endpoints.
	//
	// Local development keeps working: the check only bites once FRONTEND_URL
	// points somewhere other than localhost, which is the signal that this is a
	// real deployment.
	if c.isLocalDevelopment() {
		return nil
	}

	var insecure []string
	if c.JWTSecret == defaultJWTSecret {
		insecure = append(insecure, "JWT_SECRET")
	}
	if c.IngestAPIKey == defaultIngestAPIKey {
		insecure = append(insecure, "INGEST_API_KEY")
	}
	if len(insecure) > 0 {
		return fmt.Errorf("%s still set to the public default value(s) committed to this repository. "+
			"Set them to real secrets (openssl rand -base64 48), or set FRONTEND_URL to a localhost "+
			"address for local development", strings.Join(insecure, " and "))
	}

	return nil
}

// InsecureDefaultsWarning returns a message when the process is running locally
// with placeholder secrets, or "" when there is nothing to say. Local
// development is allowed to use the defaults, but should still be told.
func (c *Config) InsecureDefaultsWarning() string {
	if !c.isLocalDevelopment() {
		return ""
	}
	if c.JWTSecret == defaultJWTSecret {
		return "JWT_SECRET is using the public default value; set it to a real secret before deploying"
	}
	return ""
}

// isLocalDevelopment reports whether this process looks like a developer
// machine rather than a deployment, based on where the frontend lives.
func (c *Config) isLocalDevelopment() bool {
	host := c.FrontendURL
	if u, err := url.Parse(c.FrontendURL); err == nil && u.Host != "" {
		host = u.Hostname()
	}
	switch host {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	return strings.HasPrefix(c.FrontendURL, "http://localhost") ||
		strings.HasPrefix(c.FrontendURL, "http://127.0.0.1")
}
