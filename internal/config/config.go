package config

import (
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env                string
	Port               string
	DatabaseURL        string
	JWTSecret          string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	CORSAllowedOrigins []string
	AutoMigrate        bool
	LogLevel           string
	TrustedProxy       string
	CookieSecure       bool
}

func Load() Config {
	_ = godotenv.Load()
	cfg := Config{
		Env:                env("APP_ENV", env("ENV", "development")),
		Port:               env("PORT", "8080"),
		DatabaseURL:        env("DATABASE_URL", ""),
		JWTSecret:          env("JWT_SECRET", ""),
		AccessTokenTTL:     parseDuration(env("ACCESS_TOKEN_TTL", "15m"), 15*time.Minute),
		RefreshTokenTTL:    parseDuration(env("REFRESH_TOKEN_TTL", "720h"), 30*24*time.Hour),
		CORSAllowedOrigins: splitCSV(env("CORS_ALLOWED_ORIGINS", "")),
		AutoMigrate:        envBool("AUTO_MIGRATE", true),
		LogLevel:           env("LOG_LEVEL", "info"),
		TrustedProxy:       env("TRUSTED_PROXY", ""),
		CookieSecure:       envBool("COOKIE_SECURE", false),
	}
	return cfg
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return strings.EqualFold(value, "true") || value == "1"
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
