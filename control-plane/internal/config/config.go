package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"strings"
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

	AllowDevAuth               bool
	InternalAuthToken          string
	AuthCacheTTL               time.Duration
	AuthCacheMaxEntries        int
	AutoProvisionDefaultTenant bool

	// ImageProviderAPIKeyEnvs is the server-controlled provider→env-var map
	// used by image integrations. NOTE: field added here to complete the
	// parallel in-flight image-integration work (which referenced it from
	// Load() but did not declare it); audit foundation work did not author
	// this field's semantics.
	ImageProviderAPIKeyEnvs map[string]string

	// Audit tunes the bounded background audit writer (queue/batch/retry/
	// flush). The writer is best-effort: a saturated queue or exhausted
	// retries drop events with observable, secret-free evidence rather
	// than blocking user requests.
	Audit AuditConfig
}

// AuditConfig configures the audit recorder/writer lifecycle.
type AuditConfig struct {
	// QueueSize is the bounded in-process channel depth.
	QueueSize int
	// BatchSize is the maximum events appended per flush.
	BatchSize int
	// MaxRetries is the number of bounded-backoff retry attempts before a
	// failed write is reported lost.
	MaxRetries int
	// FlushInterval bounds the wait to fill a batch before flushing a
	// partial one.
	FlushInterval time.Duration
	// RetryBaseBackoff is the initial retry delay (doubled per attempt).
	RetryBaseBackoff time.Duration
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

		AllowDevAuth:               allowDevAuth,
		InternalAuthToken:          envStr("HARPIA_INTERNAL_AUTH_TOKEN", ""),
		AuthCacheTTL:               envDuration("HARPIA_AUTH_CACHE_TTL", 60*time.Second),
		AuthCacheMaxEntries:        envInt("HARPIA_AUTH_CACHE_MAX_ENTRIES", 1024),
		AutoProvisionDefaultTenant: envBool("HARPIA_AUTO_PROVISION_DEFAULT_TENANT", allowDevAuth),
		ImageProviderAPIKeyEnvs:    imageProviderAPIKeyEnvs(),

		Audit: AuditConfig{
			QueueSize:        envInt("HARPIA_AUDIT_QUEUE_SIZE", 1024),
			BatchSize:        envInt("HARPIA_AUDIT_BATCH_SIZE", 32),
			MaxRetries:       envInt("HARPIA_AUDIT_MAX_RETRIES", 3),
			FlushInterval:    envDuration("HARPIA_AUDIT_FLUSH_INTERVAL", 2*time.Second),
			RetryBaseBackoff: envDuration("HARPIA_AUDIT_RETRY_BASE_BACKOFF", 250*time.Millisecond),
		},
	}
}

// imageProviderAPIKeyEnvs returns the server-controlled provider-to-environment
// mapping used by image integrations. Tenant installation config must never
// select an environment variable. HARPIA_IMAGE_PROVIDER_API_KEY_ENVS may
// override or extend the JSON object in deployment configuration.
func imageProviderAPIKeyEnvs() map[string]string {
	envs := map[string]string{"openai": "OPENAI_API_KEY"}
	raw := strings.TrimSpace(os.Getenv("HARPIA_IMAGE_PROVIDER_API_KEY_ENVS"))
	if raw == "" {
		return envs
	}

	var overrides map[string]string
	if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
		slog.Warn("image provider API key envs override is malformed JSON; using defaults",
			"env_var", "HARPIA_IMAGE_PROVIDER_API_KEY_ENVS",
			"error", err,
		)
		return envs
	}
	for provider, envName := range overrides {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" {
			continue
		}
		envs[provider] = strings.TrimSpace(envName)
	}
	return envs
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
