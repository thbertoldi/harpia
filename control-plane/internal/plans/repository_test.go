package plans

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestListPlanConfigurationsQueryFiltersKind(t *testing.T) {
	tenantID := uuid.New()

	tests := []struct {
		name string
		kind string
	}{
		{name: "one-shot", kind: ConfigurationKindOneShot},
		{name: "recurring", kind: ConfigurationKindRecurring},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := listConfigurationsQuery(tenantID, nil, nil, "", tt.kind, 51, 0)

			if !strings.Contains(query, "kind = $2") {
				t.Fatalf("query = %q, want kind predicate", query)
			}
			if len(args) != 4 {
				t.Fatalf("args length = %d, want 4", len(args))
			}
			if args[1] != tt.kind {
				t.Fatalf("kind arg = %v, want %s", args[1], tt.kind)
			}
		})
	}
}
