package config

import (
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Port     int
	LogLevel slog.Level

	DatabaseURL  string
	ValkeyURL    string
	TemporalHost string
	GarageURL    string
	ZitadelURL   string
}

func Load() *Config {
	return &Config{
		Port:     envInt("PORT", 8080),
		LogLevel: parseLogLevel(envStr("LOG_LEVEL", "info")),

		DatabaseURL:  envStr("DATABASE_URL", "postgres://harpia:harpia@localhost:15000/harpia?sslmode=disable"),
		ValkeyURL:    envStr("VALKEY_URL", "valkey://localhost:15001"),
		TemporalHost: envStr("TEMPORAL_HOST", "localhost:7233"),
		GarageURL:    envStr("GARAGE_URL", "http://localhost:15002"),
		ZitadelURL:   envStr("ZITADEL_URL", "http://localhost:9980"),
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
