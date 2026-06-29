package linkedin

import (
	"encoding/json"
	"testing"
)

func TestParseInstallationConfigSuccess(t *testing.T) {
	config, err := ParseInstallationConfig(json.RawMessage(`{"oauth_credential_id":"cred-123"}`))
	if err != nil {
		t.Fatalf("ParseInstallationConfig() error = %v", err)
	}
	if config.OAuthCredentialID != "cred-123" {
		t.Fatalf("OAuthCredentialID = %q, want cred-123", config.OAuthCredentialID)
	}
}

func TestParseInstallationConfigApprovalOnly(t *testing.T) {
	config, err := ParseInstallationConfig(json.RawMessage(`{"mode":"approval_only"}`))
	if err != nil {
		t.Fatalf("ParseInstallationConfig() error = %v", err)
	}
	if config.Mode != ModeApprovalOnly {
		t.Fatalf("Mode = %q, want %q", config.Mode, ModeApprovalOnly)
	}
	if config.OAuthCredentialID != "" {
		t.Fatalf("OAuthCredentialID = %q, want empty", config.OAuthCredentialID)
	}
}

func TestParseInstallationConfigRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{name: "empty config", raw: `{}`},
		{name: "invalid json", raw: `{`},
		{name: "missing credential id", raw: `{"oauth_credential_id":""}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseInstallationConfig(json.RawMessage(tc.raw)); err == nil {
				t.Fatal("expected invalid config error")
			}
		})
	}
}
