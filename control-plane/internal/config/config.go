package config

import (
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Port     int
	GinMode  string
	LogLevel slog.Level

	DatabaseURL  string
	NATSURL      string
	TemporalHost string
	GarageURL    string
	AuthentikURL string
}

func Load() *Config {
	return &Config{
		Port:     envInt("PORT", 8080),
		GinMode:  envStr("GIN_MODE", "debug"),
		LogLevel: parseLogLevel(envStr("LOG_LEVEL", "info")),

		DatabaseURL:  envStr("DATABASE_URL", "postgres://localhost:5432/harpia"),
		NATSURL:      envStr("NATS_URL", "nats://localhost:4222"),
		TemporalHost: envStr("TEMPORAL_HOST", "localhost:7233"),
		GarageURL:    envStr("GARAGE_URL", "http://localhost:3900"),
		AuthentikURL: envStr("AUTHENTIK_URL", "http://localhost:9000"),
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
