package audit

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	identityv1 "github.com/harpia/control-plane/gen/harpia/identity/v1"
	"github.com/harpia/control-plane/internal/identity"
)

type recordingRecorder struct{ drafts []EventDraft }

func (r *recordingRecorder) Record(_ context.Context, draft EventDraft) error {
	r.drafts = append(r.drafts, draft)
	return nil
}

func TestAuditInterceptorDevAuth(t *testing.T) {
	tenantID := uuid.New()
	cases := []struct {
		name          string
		allowDevAuth  bool
		authorization string
		nextError     error
		wantEvents    int
	}{
		{name: "enabled dev token successful", allowDevAuth: true, authorization: "Bearer dev-token", wantEvents: 1},
		{name: "enabled dev token rejected request", allowDevAuth: true, authorization: "Bearer dev-token", nextError: connect.NewError(connect.CodeInvalidArgument, context.Canceled), wantEvents: 0},
		{name: "disabled dev token", allowDevAuth: false, authorization: "Bearer dev-token", wantEvents: 0},
		{name: "normal bearer", allowDevAuth: true, authorization: "Bearer bearer-token", wantEvents: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &recordingRecorder{}
			auditInterceptor := NewAuditInterceptor(recorder)
			identityInterceptor := identity.NewRequestContextInterceptor(identity.AuthOptions{
				DevTenantID:   tenantID,
				AllowDevAuth:  tc.allowDevAuth,
				Authenticator: interceptorAuthenticator{},
				Memberships:   interceptorMemberships{tenantID: tenantID},
			})
			next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
				if tc.nextError != nil {
					return nil, tc.nextError
				}
				return connect.NewResponse(&identityv1.GetTenantResponse{}), nil
			})
			req := connect.NewRequest(&identityv1.GetTenantRequest{TenantId: identity.DevTenantAlias})
			req.Header().Set("Authorization", tc.authorization)
			req.Header().Set("X-Tenant-ID", identity.DevTenantAlias)
			req.Header().Set("X-Trace-ID", "trace-123")
			_, _ = identityInterceptor.WrapUnary(auditInterceptor.WrapUnary(next))(context.Background(), req)

			if got := len(recorder.drafts); got != tc.wantEvents {
				t.Fatalf("recorded events = %d, want %d", got, tc.wantEvents)
			}
			if tc.wantEvents == 1 {
				draft := recorder.drafts[0]
				if draft.EventType != eventAuthenticationDevAuth || draft.TenantID != tenantID {
					t.Fatalf("unexpected event: %#v", draft)
				}
				if draft.Actor.Kind != actorKindHuman || draft.Actor.ID == "" || draft.TraceID != "trace-123" {
					t.Fatalf("missing resolved actor/trace: %#v", draft)
				}
			}
		})
	}
}

func TestExistingOperationRegistry(t *testing.T) {
	for _, operation := range existingOperations {
		got, ok := operationForProcedure(operation.Procedure)
		if !ok || got != operation {
			t.Fatalf("operation %q is not retrievable", operation.Procedure)
		}
	}
	if _, ok := operationForProcedure("/unknown.Service/Mutation"); ok {
		t.Fatal("unknown procedure must not be audited")
	}
}

func TestAuditInterceptorConfigJSONDiffIsRedacted(t *testing.T) {
	diff := redactDiff([]DiffEntry{{
		Field: "config_json", After: `{"oauth_token":"never-store-me"}`, HasAfter: true,
	}}, []string{"config_json"})
	if len(diff) != 1 || diff[0].After != redactedPlaceholder || diff[0].Before != redactedPlaceholder {
		t.Fatalf("config_json was not redacted: %#v", diff)
	}
}

type interceptorAuthenticator struct{}

func (interceptorAuthenticator) AuthenticateBearer(context.Context, string) (identity.AuthenticatedUser, error) {
	return identity.AuthenticatedUser{Subject: "bearer-user"}, nil
}

type interceptorMemberships struct{ tenantID uuid.UUID }

func (m interceptorMemberships) ResolveMemberships(context.Context, identity.AuthenticatedUser) ([]identity.TenantMembership, error) {
	return []identity.TenantMembership{{TenantID: m.tenantID, Slug: identity.DevTenantAlias, Role: "member"}}, nil
}
