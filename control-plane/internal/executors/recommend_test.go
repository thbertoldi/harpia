package executors

import (
	"sort"
	"strings"
	"testing"
)

// testSKU builds an ExecutorSKU inline for recommendation tests.
func testSKU(key, name, kind, tier string, caps ...string) *ExecutorSKU {
	return &ExecutorSKU{
		Key:         key,
		DisplayName: name,
		Kind:        kind,
		Compatibility: CompatibilityMetadata{
			Capabilities: caps,
			Tier:         tier,
		},
	}
}

// coveredCaps returns the union of capabilities declared by a team.
func coveredCaps(team []*ExecutorSKU) map[string]struct{} {
	out := map[string]struct{}{}
	for _, s := range team {
		for _, c := range s.Compatibility.Capabilities {
			out[strings.TrimSpace(c)] = struct{}{}
		}
	}
	return out
}

func teamHasKey(team []*ExecutorSKU, key string) bool {
	for _, s := range team {
		if s.Key == key {
			return true
		}
	}
	return false
}

func TestRecommendTeamCoversWithOneMultiCapableAgent(t *testing.T) {
	specialist := testSKU(
		"linkedin-content-specialist",
		"LinkedIn Content Specialist",
		KindAgent, "senior",
		"linkedin-content-adaptation", "carousel-authoring",
	)
	singleCap := testSKU(
		"linkedin-voice",
		"LinkedIn Voice",
		KindAgent, "senior",
		"linkedin-content-adaptation",
	)

	rec := RecommendTeam(
		[]string{"linkedin-content-adaptation", "carousel-authoring"},
		[]*ExecutorSKU{specialist, singleCap},
	)

	if len(rec.Team) != 1 {
		t.Fatalf("team size = %d, want 1 (one multi-capable agent covers both): %+v", len(rec.Team), keysOf(rec.Team))
	}
	if !teamHasKey(rec.Team, "linkedin-content-specialist") {
		t.Fatalf("expected the specialist in the team, got keys %v", keysOf(rec.Team))
	}
	if len(rec.Uncovered) != 0 {
		t.Fatalf("expected no uncovered capabilities, got %v", rec.Uncovered)
	}
}

func TestRecommendTeamFlagsUncovered(t *testing.T) {
	agent := testSKU("agent-a", "Agent A", KindAgent, "senior", "covered-cap")

	rec := RecommendTeam(
		[]string{"covered-cap", "no-agent-has-this"},
		[]*ExecutorSKU{agent},
	)

	// The covered capability must be satisfied by the team.
	if !teamHasKey(rec.Team, "agent-a") {
		t.Fatalf("expected agent-a in the team, got keys %v", keysOf(rec.Team))
	}
	caps := coveredCaps(rec.Team)
	if _, ok := caps["covered-cap"]; !ok {
		t.Fatalf("team does not cover 'covered-cap': %v", caps)
	}

	// The capability no agent provides must be flagged, not silently satisfied.
	found := false
	for _, c := range rec.Uncovered {
		if c == "no-agent-has-this" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'no-agent-has-this' in Uncovered, got %v", rec.Uncovered)
	}
	// And the covered one must not appear in Uncovered.
	for _, c := range rec.Uncovered {
		if c == "covered-cap" {
			t.Fatalf("'covered-cap' should be covered, not in Uncovered")
		}
	}
}

func TestRecommendTeamPrefersFewerAgents(t *testing.T) {
	ab := testSKU("agent-ab", "Agent AB", KindAgent, "senior", "a", "b")
	c := testSKU("agent-c", "Agent C", KindAgent, "senior", "c")
	abc := testSKU("agent-abc", "Agent ABC", KindAgent, "senior", "a", "b", "c")

	rec := RecommendTeam(
		[]string{"a", "b", "c"},
		[]*ExecutorSKU{ab, c, abc},
	)

	// Robust to the greedy choice: assert a minimal cover (<= 2 agents) with
	// complete coverage, rather than pinning the exact selection.
	if len(rec.Team) > 2 {
		t.Fatalf("team size = %d, want a minimal cover of <= 2: %v", len(rec.Team), keysOf(rec.Team))
	}
	if len(rec.Uncovered) != 0 {
		t.Fatalf("expected full coverage, got uncovered %v", rec.Uncovered)
	}
	caps := coveredCaps(rec.Team)
	for _, want := range []string{"a", "b", "c"} {
		if _, ok := caps[want]; !ok {
			t.Fatalf("team does not cover %q: %v", want, caps)
		}
	}
}

func TestRecommendTeamPrefersSeniorTier(t *testing.T) {
	junior := testSKU("role-junior", "Same Role", KindAgent, "junior", "shared-cap")
	senior := testSKU("role-senior", "Same Role", KindAgent, "senior", "shared-cap")

	rec := RecommendTeam([]string{"shared-cap"}, []*ExecutorSKU{junior, senior})

	if len(rec.Team) != 1 {
		t.Fatalf("team size = %d, want 1: %v", len(rec.Team), keysOf(rec.Team))
	}
	if got := rec.Team[0].Compatibility.Tier; got != "senior" {
		t.Fatalf("picked tier = %q, want senior", got)
	}
	if !teamHasKey(rec.Team, "role-senior") {
		t.Fatalf("expected the senior-tier agent, got keys %v", keysOf(rec.Team))
	}
}

func TestRecommendTeamIgnoresIntegrationsAndCapabilityless(t *testing.T) {
	// An integration that happens to declare the required capability must be
	// ignored (only agents are team candidates).
	integration := testSKU("rss-feed", "RSS Feed", KindIntegration, "", "a")
	// An agent with no capabilities must be ignored too.
	capabilityless := testSKU("agent-empty", "Empty Agent", KindAgent, "senior")
	// The only valid candidate for the required capability.
	agent := testSKU("agent-a", "Agent A", KindAgent, "senior", "a")

	rec := RecommendTeam(
		[]string{"a"},
		[]*ExecutorSKU{integration, capabilityless, agent},
	)

	if len(rec.Team) != 1 {
		t.Fatalf("team size = %d, want exactly the one capable agent: %v", len(rec.Team), keysOf(rec.Team))
	}
	if !teamHasKey(rec.Team, "agent-a") {
		t.Fatalf("expected only agent-a selected, got keys %v", keysOf(rec.Team))
	}
	if teamHasKey(rec.Team, "rss-feed") {
		t.Fatalf("integration sku must never be selected into an agent team")
	}
	if teamHasKey(rec.Team, "agent-empty") {
		t.Fatalf("capabilityless agent must never be selected into an agent team")
	}
	if len(rec.Uncovered) != 0 {
		t.Fatalf("expected full coverage, got uncovered %v", rec.Uncovered)
	}
}

// keysOf returns the sorted SKU keys of a team, for readable failure messages.
func keysOf(team []*ExecutorSKU) []string {
	out := make([]string, 0, len(team))
	for _, s := range team {
		out = append(out, s.Key)
	}
	sort.Strings(out)
	return out
}
