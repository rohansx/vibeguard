package config

import "os"

type Config struct {
	DatabaseURL         string
	ClerkSecretKey      string
	OpenAIAPIKey        string
	IngestionServiceURL string
	SentryDSN           string
}

func Load() *Config {
	return &Config{
		DatabaseURL:         getenv("DATABASE_URL", "postgres://vibeguard:vibeguard@localhost:5432/vibeguard?sslmode=disable"),
		ClerkSecretKey:      getenv("CLERK_SECRET_KEY", ""),
		OpenAIAPIKey:        getenv("OPENAI_API_KEY", ""),
		IngestionServiceURL: getenv("INGESTION_SERVICE_URL", "http://localhost:8001"),
		SentryDSN:           getenv("SENTRY_DSN", ""),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
