package rss

import (
	"encoding/json"
	"testing"
)

func TestParseInstallationConfigSuccess(t *testing.T) {
	config, err := ParseInstallationConfig(json.RawMessage(`{"feeds":["https://example.com/rss","https://news.example/rss.xml"]}`))
	if err != nil {
		t.Fatalf("ParseInstallationConfig() error = %v", err)
	}
	if len(config.Feeds) != 2 {
		t.Fatalf("Feeds = %#v, want 2 entries", config.Feeds)
	}
}

func TestParseInstallationConfigRejectsInvalidFeedInput(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty config", `{}`},
		{"empty feeds array", `{"feeds":[]}`},
		{"invalid json", `{`},
		{"relative url", `{"feeds":["/rss.xml"]}`},
		{"empty feed url", `{"feeds":[""]}`},
		{"unsupported scheme", `{"feeds":["ftp://example.com/rss"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseInstallationConfig(json.RawMessage(tc.raw)); err == nil {
				t.Fatal("expected invalid config error")
			}
		})
	}
}

func TestParseInstallationConfigDedupesFeeds(t *testing.T) {
	config, err := ParseInstallationConfig(json.RawMessage(`{"feeds":["https://example.com/rss","https://example.com/rss"]}`))
	if err != nil {
		t.Fatalf("ParseInstallationConfig() error = %v", err)
	}
	if len(config.Feeds) != 1 {
		t.Fatalf("Feeds = %#v, want one deduped entry", config.Feeds)
	}
}
