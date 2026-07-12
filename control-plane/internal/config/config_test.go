package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoadAuditDefaults(t *testing.T) {
	t.Setenv("HARPIA_AUDIT_QUEUE_SIZE", "")
	t.Setenv("HARPIA_AUDIT_BATCH_SIZE", "")
	t.Setenv("HARPIA_AUDIT_MAX_RETRIES", "")
	t.Setenv("HARPIA_AUDIT_FLUSH_INTERVAL", "")
	t.Setenv("HARPIA_AUDIT_RETRY_BASE_BACKOFF", "")

	cfg := Load()

	if cfg.Audit.QueueSize != 1024 {
		t.Fatalf("Audit.QueueSize = %d, want 1024", cfg.Audit.QueueSize)
	}
	if cfg.Audit.BatchSize != 32 {
		t.Fatalf("Audit.BatchSize = %d, want 32", cfg.Audit.BatchSize)
	}
	if cfg.Audit.MaxRetries != 3 {
		t.Fatalf("Audit.MaxRetries = %d, want 3", cfg.Audit.MaxRetries)
	}
	if cfg.Audit.FlushInterval != 2*time.Second {
		t.Fatalf("Audit.FlushInterval = %v, want 2s", cfg.Audit.FlushInterval)
	}
	if cfg.Audit.RetryBaseBackoff != 250*time.Millisecond {
		t.Fatalf("Audit.RetryBaseBackoff = %v, want 250ms", cfg.Audit.RetryBaseBackoff)
	}
}

func TestLoadAuditOverrides(t *testing.T) {
	t.Setenv("HARPIA_AUDIT_QUEUE_SIZE", "2048")
	t.Setenv("HARPIA_AUDIT_BATCH_SIZE", "64")
	t.Setenv("HARPIA_AUDIT_MAX_RETRIES", "5")
	t.Setenv("HARPIA_AUDIT_FLUSH_INTERVAL", "500ms")
	t.Setenv("HARPIA_AUDIT_RETRY_BASE_BACKOFF", "100ms")

	cfg := Load()

	if cfg.Audit.QueueSize != 2048 {
		t.Fatalf("Audit.QueueSize = %d, want 2048", cfg.Audit.QueueSize)
	}
	if cfg.Audit.BatchSize != 64 {
		t.Fatalf("Audit.BatchSize = %d, want 64", cfg.Audit.BatchSize)
	}
	if cfg.Audit.MaxRetries != 5 {
		t.Fatalf("Audit.MaxRetries = %d, want 5", cfg.Audit.MaxRetries)
	}
	if cfg.Audit.FlushInterval != 500*time.Millisecond {
		t.Fatalf("Audit.FlushInterval = %v, want 500ms", cfg.Audit.FlushInterval)
	}
	if cfg.Audit.RetryBaseBackoff != 100*time.Millisecond {
		t.Fatalf("Audit.RetryBaseBackoff = %v, want 100ms", cfg.Audit.RetryBaseBackoff)
	}
}

// TestImageProviderAPIKeyEnvsMalformedJSONWarns asserts that a malformed
// HARPIA_IMAGE_PROVIDER_API_KEY_ENVS override emits a warning log (so an
// operator's misconfiguration is observable) while still falling back to the
// safe default mapping.
func TestImageProviderAPIKeyEnvsMalformedJSONWarns(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	origDefault := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(origDefault) })

	t.Setenv("HARPIA_IMAGE_PROVIDER_API_KEY_ENVS", "{not-json")

	cfg := Load()

	// Fail-safe: the default mapping is still present despite the bad override.
	if got := cfg.ImageProviderAPIKeyEnvs["openai"]; got != "OPENAI_API_KEY" {
		t.Fatalf("openai env = %q, want OPENAI_API_KEY", got)
	}
	out := buf.String()
	if !strings.Contains(out, "malformed JSON") {
		t.Fatalf("expected warning about malformed JSON, got: %s", out)
	}
	if !strings.Contains(out, "using defaults") {
		t.Fatalf("expected warning to mention using defaults, got: %s", out)
	}
}
