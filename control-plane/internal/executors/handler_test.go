package executors

import (
	"testing"

	executorsv1 "github.com/harpia/control-plane/gen/harpia/executors/v1"
)

func TestDBKindRoundTrip(t *testing.T) {
	cases := []struct {
		db   string
		proto executorsv1.ExecutorKind
	}{
		{KindIntegration, executorsv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION},
		{KindAgent, executorsv1.ExecutorKind_EXECUTOR_KIND_AGENT},
	}

	for _, tc := range cases {
		if got := dbKindToProto(tc.db); got != tc.proto {
			t.Fatalf("dbKindToProto(%q) = %v, want %v", tc.db, got, tc.proto)
		}
		kind := tc.proto
		got, err := protoKindToDB(&kind)
		if err != nil {
			t.Fatalf("protoKindToDB(%v): %v", tc.proto, err)
		}
		if got != tc.db {
			t.Fatalf("protoKindToDB(%v) = %q, want %q", tc.proto, got, tc.db)
		}
	}
}

func TestConnectionStatusRoundTrip(t *testing.T) {
	cases := []struct {
		db    string
		proto executorsv1.ConnectionStatus
	}{
		{"disconnected", executorsv1.ConnectionStatus_CONNECTION_STATUS_DISCONNECTED},
		{"connecting", executorsv1.ConnectionStatus_CONNECTION_STATUS_CONNECTING},
		{"connected", executorsv1.ConnectionStatus_CONNECTION_STATUS_CONNECTED},
		{"error", executorsv1.ConnectionStatus_CONNECTION_STATUS_ERROR},
	}

	for _, tc := range cases {
		if got := connectionStatusToProto(tc.db); got != tc.proto {
			t.Fatalf("connectionStatusToProto(%q) = %v, want %v", tc.db, got, tc.proto)
		}
		got, err := connectionStatusToDB(tc.proto)
		if err != nil {
			t.Fatalf("connectionStatusToDB(%v): %v", tc.proto, err)
		}
		if got != tc.db {
			t.Fatalf("connectionStatusToDB(%v) = %q, want %q", tc.proto, got, tc.db)
		}
	}
}

func TestPageParamsDefaults(t *testing.T) {
	limit, offset, err := pageParams(0, "")
	if err != nil {
		t.Fatalf("pageParams: %v", err)
	}
	if limit != 50 || offset != 0 {
		t.Fatalf("got limit=%d offset=%d, want 50/0", limit, offset)
	}
}

func TestPageParamsInvalidToken(t *testing.T) {
	_, _, err := pageParams(10, "bad-token")
	if err == nil {
		t.Fatal("expected invalid page token error")
	}
}
