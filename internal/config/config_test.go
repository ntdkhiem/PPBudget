package config

import (
	"strings"
	"testing"
)

func TestIsLocalDevelopment(t *testing.T) {
	tests := []struct {
		frontendURL string
		want        bool
	}{
		{"http://localhost:3000", true},
		{"http://localhost", true},
		{"http://127.0.0.1:3000", true},
		{"https://127.0.0.1", true},
		{"http://0.0.0.0:3000", true},

		{"https://ppbudget.vercel.app", false},
		{"https://budget.example.com", false},
		// A hostname merely containing "localhost" is not localhost.
		{"https://localhost.evil.example.com", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.frontendURL, func(t *testing.T) {
			c := &Config{FrontendURL: tt.frontendURL}
			if got := c.isLocalDevelopment(); got != tt.want {
				t.Errorf("isLocalDevelopment(%q) = %v, want %v", tt.frontendURL, got, tt.want)
			}
		})
	}
}

// TestDefaultsAreDetected guards the string comparison MustLoad relies on: if a
// default is edited in one place and not the other, the startup check silently
// stops protecting anything.
func TestDefaultsAreDetected(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example/db")
	cfg := Load()

	if cfg.JWTSecret != defaultJWTSecret {
		t.Errorf("Load() with JWT_SECRET unset gave %q, want the declared default %q", cfg.JWTSecret, defaultJWTSecret)
	}
	if cfg.IngestAPIKey != defaultIngestAPIKey {
		t.Errorf("Load() with INGEST_API_KEY unset gave %q, want the declared default %q", cfg.IngestAPIKey, defaultIngestAPIKey)
	}

	t.Setenv("JWT_SECRET", "a-real-secret")
	if got := Load().JWTSecret; got != "a-real-secret" {
		t.Errorf("Load() ignored JWT_SECRET from the environment, got %q", got)
	}
}

// TestValidate covers the startup gate: a deployment must not boot with the
// placeholder secrets committed to this repository, while local development
// must stay frictionless.
func TestValidate(t *testing.T) {
	valid := func() *Config {
		return &Config{
			DatabaseURL:  "postgres://example/db",
			JWTSecret:    "a-real-secret",
			IngestAPIKey: "a-real-key",
			FrontendURL:  "https://ppbudget.vercel.app",
		}
	}

	tests := []struct {
		name      string
		mutate    func(c *Config)
		wantErr   bool
		errorMust string
	}{
		{"fully configured deployment", func(c *Config) {}, false, ""},
		{"missing database url", func(c *Config) { c.DatabaseURL = "" }, true, "DATABASE_URL"},

		{"deployment with default jwt secret", func(c *Config) {
			c.JWTSecret = defaultJWTSecret
		}, true, "JWT_SECRET"},
		{"deployment with default ingest key", func(c *Config) {
			c.IngestAPIKey = defaultIngestAPIKey
		}, true, "INGEST_API_KEY"},
		{"deployment with both defaults names both", func(c *Config) {
			c.JWTSecret = defaultJWTSecret
			c.IngestAPIKey = defaultIngestAPIKey
		}, true, "JWT_SECRET and INGEST_API_KEY"},

		// Local development keeps working with the placeholders.
		{"localhost with default secrets is allowed", func(c *Config) {
			c.FrontendURL = "http://localhost:3000"
			c.JWTSecret = defaultJWTSecret
			c.IngestAPIKey = defaultIngestAPIKey
		}, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.mutate(c)

			err := c.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected Validate to reject this configuration")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected Validate to accept this configuration, got %v", err)
			}
			if err != nil && tt.errorMust != "" && !strings.Contains(err.Error(), tt.errorMust) {
				t.Errorf("error %q should name %q so the operator knows what to fix", err, tt.errorMust)
			}
		})
	}
}

func TestInsecureDefaultsWarning(t *testing.T) {
	local := &Config{FrontendURL: "http://localhost:3000", JWTSecret: defaultJWTSecret}
	if local.InsecureDefaultsWarning() == "" {
		t.Error("local development with the default JWT secret should warn")
	}

	localOK := &Config{FrontendURL: "http://localhost:3000", JWTSecret: "a-real-secret"}
	if got := localOK.InsecureDefaultsWarning(); got != "" {
		t.Errorf("a configured local setup should not warn, got %q", got)
	}

	// A deployment is refused by Validate, so the warning path stays quiet.
	deployed := &Config{FrontendURL: "https://example.com", JWTSecret: defaultJWTSecret}
	if got := deployed.InsecureDefaultsWarning(); got != "" {
		t.Errorf("deployments are handled by Validate, not the warning, got %q", got)
	}
}
