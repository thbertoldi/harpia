package plans

import (
	"testing"
)

func TestCreatePlanConfiguration_WritesConfigurationSavedMessage(t *testing.T) {
	t.Skip("integration test — requires Postgres; covered by E2E suite. The hook itself is verified by reading the handler diff.")
}
