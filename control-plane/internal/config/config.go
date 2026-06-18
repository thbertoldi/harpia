package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port     int
	LogLevel slog.Level

	DatabaseURL     string
	ValkeyURL       string
	TemporalHost    string
	GarageURL       string
	GarageBucket    string
	GarageAccessKey string
	GarageSecretKey string
	GarageRegion    string
	ZitadelURL      string
	ZitadelHost     string
	OpenFGAURL      string

	OpenFGAStoreID              string
	OpenFGAAuthorizationModelID string

	AllowDevAuth               bool
	InternalAuthToken          string
	AuthCacheTTL               time.Duration
	AuthCacheMaxEntries        int
	AutoProvisionDefaultTenant bool
}

func Load() *Config {
	allowDevAuth := envBool("HARPIA_ALLOW_DEV_AUTH", false)
	return &Config{
		Port:     envInt("PORT", 8080),
		LogLevel: parseLogLevel(envStr("LOG_LEVEL", "info")),

		DatabaseURL:     envStr("DATABASE_URL", "postgres://harpia:harpia@localhost:15000/harpia?sslmode=disable"),
		ValkeyURL:       envStr("VALKEY_URL", "valkey://localhost:15001"),
		TemporalHost:    envStr("TEMPORAL_HOST", "localhost:7233"),
		GarageURL:       envStr("GARAGE_URL", "http://localhost:15002"),
		GarageBucket:    envStr("GARAGE_BUCKET", "harpia"),
		GarageAccessKey: envStr("GARAGE_ACCESS_KEY", ""),
		GarageSecretKey: envStr("GARAGE_SECRET_KEY", ""),
		GarageRegion:    envStr("GARAGE_REGION", "garage"),
		ZitadelURL:      envStr("ZITADEL_URL", "http://localhost:9980"),
		ZitadelHost:     envStr("ZITADEL_HOST", ""),
		OpenFGAURL:      envStr("OPENFGA_API_URL", "http://localhost:18086"),

		OpenFGAStoreID:              envStr("OPENFGA_STORE_ID", ""),
		OpenFGAAuthorizationModelID: envStr("OPENFGA_AUTHORIZATION_MODEL_ID", ""),

		AllowDevAuth:               allowDevAuth,
		InternalAuthToken:          envStr("HARPIA_INTERNAL_AUTH_TOKEN", ""),
		AuthCacheTTL:               envDuration("HARPIA_AUTH_CACHE_TTL", 60*time.Second),
		AuthCacheMaxEntries:        envInt("HARPIA_AUTH_CACHE_MAX_ENTRIES", 1024),
		AutoProvisionDefaultTenant: envBool("HARPIA_AUTO_PROVISION_DEFAULT_TENANT", allowDevAuth),
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
