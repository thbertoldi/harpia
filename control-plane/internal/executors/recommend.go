package executors

import "strings"

// TeamRecommendation is the recommended agent team for a set of required
// capabilities (ADR-018 D3). Team is a minimal covering set of agent SKUs;
// Uncovered lists required capabilities no candidate agent provides.
type TeamRecommendation struct {
	Team      []*ExecutorSKU
	Uncovered []string
}

// RecommendTeam computes a minimal set of agent SKUs whose combined
// capabilities cover requiredCapabilities, using greedy set-cover.
//
// Only agent-kind SKUs that declare at least one capability are candidates.
// Ties prefer the senior tier, then fewer surplus capabilities, then display
// name (deterministic). Required capabilities covered by no candidate end up in
// Uncovered. The input sku order does not affect the result beyond the
// documented tie-breaks.
func RecommendTeam(requiredCapabilities []string, skus []*ExecutorSKU) TeamRecommendation {
	required := normalizeCapabilities(requiredCapabilities)
	requiredSet := make(map[string]struct{}, len(required))
	for _, c := range required {
		requiredSet[c] = struct{}{}
	}

	// Candidates: agent-kind SKUs that declare at least one capability.
	// Each candidate's capability set is normalized once; surplus (capabilities
	// outside the required set) and tier rank are fixed, so they factor only
	// into tie-breaks.
	candidates := make([]teamCandidate, 0, len(skus))
	for _, sku := range skus {
		if sku == nil || sku.Kind != KindAgent {
			continue
		}
		normalized := normalizeCapabilities(sku.Compatibility.Capabilities)
		if len(normalized) == 0 {
			continue
		}
		caps := make(map[string]struct{}, len(normalized))
		var surplus int
		for _, c := range normalized {
			caps[c] = struct{}{}
			if _, ok := requiredSet[c]; !ok {
				surplus++
			}
		}
		candidates = append(candidates, teamCandidate{
			sku:      sku,
			tierRank: tierRank(sku.Compatibility.Tier),
			caps:     caps,
			surplus:  surplus,
		})
	}

	covered := make(map[string]struct{}, len(required))
	team := make([]*ExecutorSKU, 0)
	for {
		// Pick the candidate covering the most still-uncovered required
		// capabilities; stop when no candidate can make further progress.
		var best *teamCandidate
		bestCovered := 0
		for i := range candidates {
			c := &candidates[i]
			coveredCount := 0
			for cap := range c.caps {
				if _, req := requiredSet[cap]; !req {
					continue
				}
				if _, done := covered[cap]; done {
					continue
				}
				coveredCount++
			}
			if coveredCount == 0 {
				continue
			}
			if best == nil || betterTeamCandidate(c, best, coveredCount, bestCovered) {
				best = c
				bestCovered = coveredCount
			}
		}
		if best == nil {
			break
		}
		team = append(team, best.sku)
		for cap := range best.caps {
			if _, req := requiredSet[cap]; req {
				covered[cap] = struct{}{}
			}
		}
	}

	uncovered := make([]string, 0, len(required))
	for _, c := range required {
		if _, ok := covered[c]; !ok {
			uncovered = append(uncovered, c)
		}
	}

	return TeamRecommendation{Team: team, Uncovered: uncovered}
}

// teamCandidate is a precomputed candidate for the greedy set-cover.
type teamCandidate struct {
	sku      *ExecutorSKU
	tierRank int
	caps     map[string]struct{}
	surplus  int // capabilities not in the required set
}

// betterTeamCandidate reports whether candidate c (covering coveredCount
// still-uncovered required capabilities) is a strictly better greedy pick than
// the current best b (covering bestCount). Tie-break order: more coverage,
// senior tier, fewer surplus capabilities, then display name, then key — all of
// which makes the pick fully deterministic.
func betterTeamCandidate(c, b *teamCandidate, coveredCount, bestCount int) bool {
	if coveredCount != bestCount {
		return coveredCount > bestCount
	}
	if c.tierRank != b.tierRank {
		return c.tierRank < b.tierRank
	}
	if c.surplus != b.surplus {
		return c.surplus < b.surplus
	}
	if c.sku.DisplayName != b.sku.DisplayName {
		return c.sku.DisplayName < b.sku.DisplayName
	}
	return c.sku.Key < b.sku.Key
}

// tierRank orders tiers so senior is preferred: senior=0, pleno=1, junior=2,
// anything else=3.
func tierRank(tier string) int {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "senior":
		return 0
	case "pleno":
		return 1
	case "junior":
		return 2
	default:
		return 3
	}
}

// normalizeCapabilities trims, drops blanks, and de-duplicates, preserving
// first-seen order.
func normalizeCapabilities(caps []string) []string {
	seen := make(map[string]struct{}, len(caps))
	out := make([]string, 0, len(caps))
	for _, c := range caps {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}
