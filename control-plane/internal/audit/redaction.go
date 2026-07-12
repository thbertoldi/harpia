package audit

import (
	"strings"
)

// secretFieldNames is the denylist of diff field names that are redacted
// regardless of allowlist membership. Matching is case-insensitive against
// the lowercased field name and common compound forms (e.g. nested keys
// joined by '.').
var secretFieldNames = []string{
	"password", "secret", "token", "api_key", "apikey", "apikeyid",
	"access_key", "secretkey", "private_key", "privatekey",
	"client_secret", "clientsecret", "refresh_token", "bearertoken",
	"authorization", "config_json", "credentials", "credential",
	"oauth_token", "oauth_secret", "session_token",
}

// redactedPlaceholder replaces a redacted diff value so a reviewer can see
// that a value existed but was withheld.
const redactedPlaceholder = "<redacted>"

// redactDiff applies two layers of defense before an event is persisted:
//  1. A secret-bearing field name is always redacted (denylist), so a stray
//     `config_json` or `api_key` value never reaches the ledger.
//  2. An allowlist names the safe fields we expect to audit; anything not on
//     it is dropped entirely rather than stored speculatively.
//
// The combination means even a future caller that builds a diff from an
// arbitrary struct cannot leak a credential: unknown fields are omitted and
// known-secret fields are masked.
func redactDiff(entries []DiffEntry, allowlist []string) []DiffEntry {
	if len(entries) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(allowlist))
	for _, f := range allowlist {
		allowed[strings.ToLower(strings.TrimSpace(f))] = struct{}{}
	}

	out := make([]DiffEntry, 0, len(entries))
	for _, entry := range entries {
		field := strings.TrimSpace(entry.Field)
		if field == "" {
			continue
		}
		lower := strings.ToLower(field)
		if isSecretField(lower) {
			entry.Before = redactedPlaceholder
			entry.After = redactedPlaceholder
			entry.HasBefore = true
			entry.HasAfter = true
			out = append(out, entry)
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[lower]; !ok {
				continue
			}
		}
		out = append(out, entry)
	}
	return out
}

func isSecretField(lower string) bool {
	for _, bad := range secretFieldNames {
		if lower == bad {
			return true
		}
		// Also catch compound forms like "config.api_key".
		if strings.Contains(lower, "."+bad) || strings.Contains(lower, bad+".") {
			return true
		}
	}
	return false
}
