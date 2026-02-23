package config

import "os"

type Config struct {
	DatabaseURL     string
	ClerkSecretKey  string
	AnthropicAPIKey string // Claude Sonnet for hybrid sanitizer classification (Phase 2)
	SPGStorePath    string // local graph DB path (default: ~/.vibeguard/graph)
	SentryDSN       string
}

func Load() *Config {
	return &Config{
		DatabaseURL:     getenv("DATABASE_URL", "postgres://vibeguard:vibeguard@localhost:5432/vibeguard?sslmode=disable"),
		ClerkSecretKey:  getenv("CLERK_SECRET_KEY", ""),
		AnthropicAPIKey: getenv("ANTHROPIC_API_KEY", ""),
		SPGStorePath:    getenv("SPG_STORE_PATH", ""),
		SentryDSN:       getenv("SENTRY_DSN", ""),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
