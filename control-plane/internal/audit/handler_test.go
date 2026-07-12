package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	auditv1 "github.com/harpia/control-plane/gen/harpia/audit/v1"
	"github.com/harpia/control-plane/internal/identity"
)

type stubLister struct {
	listFn func(ctx context.Context, tenantID uuid.UUID, filters Filters, cursorOccurredAt time.Time, cursorID uuid.UUID, pageSize int) ([]Event, error)
}

func (s *stubLister) List(ctx context.Context, tenantID uuid.UUID, filters Filters, cur time.Time, id uuid.UUID, pageSize int) ([]Event, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID, filters, cur, id, pageSize)
	}
	return nil, nil
}

func auditContext(tenantID uuid.UUID) context.Context {
	return identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID: tenantID,
		UserID:   "test-user",
		Roles:    []string{"engineer"},
		Tenants: []identity.TenantMembership{
			{TenantID: tenantID, Slug: "tenant-a", Role: "engineer"},
		},
	})
}

func stringPtr(s string) *string { return &s }

func decisionPtr(d auditv1.FeedbackDecision) *auditv1.FeedbackDecision { return &d }

func TestListAuditEventsRequiresAuthorizedTenant(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	other := uuid.New()
	handler := &Handler{repo: &stubLister{}, now: time.Now}

	// Requesting a tenant not in the caller's context → PermissionDenied.
	_, err := handler.ListAuditEvents(
		auditContext(tenantID),
		connect.NewRequest(&auditv1.ListAuditEventsRequest{TenantId: other.String()}),
	)
	if err == nil {
		t.Fatal("expected permission denied for foreign tenant")
	}
	if got := connect.CodeOf(err); got != connect.CodePermissionDenied {
		t.Fatalf("code = %v, want %v", got, connect.CodePermissionDenied)
	}
}

func TestListAuditEventsRejectsMalformedCursor(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{repo: &stubLister{}, now: time.Now}

	_, err := handler.ListAuditEvents(
		auditContext(tenantID),
		connect.NewRequest(&auditv1.ListAuditEventsRequest{
			TenantId:  tenantID.String(),
			PageToken: stringPtr("!!!not-a-valid-token!!!"),
		}),
	)
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want %v", got, connect.CodeInvalidArgument)
	}
}

// G5: a token minted for one filter scope must be rejected under a different
// filter set.
func TestListAuditEventsRejectsMismatchedCursorScope(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{repo: &stubLister{}, now: time.Now}

	scopeA := scopeDigest(Filters{EventTypes: []string{eventPlanExecutionStarted}})
	token := encodeCursor(time.Now().UTC(), uuid.New(), scopeA)

	_, err := handler.ListAuditEvents(
		auditContext(tenantID),
		connect.NewRequest(&auditv1.ListAuditEventsRequest{
			TenantId:  tenantID.String(),
			PageToken: stringPtr(token),
		}),
	)
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want %v", got, connect.CodeInvalidArgument)
	}
}

func TestListAuditEventsRejectsBadFilter(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{repo: &stubLister{}, now: time.Now}

	// Decision UNSPECIFIED is invalid as a filter value.
	_, err := handler.ListAuditEvents(
		auditContext(tenantID),
		connect.NewRequest(&auditv1.ListAuditEventsRequest{
			TenantId: tenantID.String(),
			Decision: decisionPtr(auditv1.FeedbackDecision_FEEDBACK_DECISION_UNSPECIFIED),
		}),
	)
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want %v", got, connect.CodeInvalidArgument)
	}
}

func TestListAuditEventsMapsEventsAndNextToken(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)

	var capturedPageSize int
	handler := &Handler{
		repo: &stubLister{
			listFn: func(_ context.Context, gotTenant uuid.UUID, _ Filters, _ time.Time, _ uuid.UUID, pageSize int) ([]Event, error) {
				capturedPageSize = pageSize
				if gotTenant != tenantID {
					t.Fatalf("tenant = %s, want %s", gotTenant, tenantID)
				}
				// Return pageSize+1 so the handler mints a next token.
				events := make([]Event, 0, pageSize+1)
				for i := 0; i < pageSize+1; i++ {
					events = append(events, Event{
						ID: uuid.New(), TenantID: tenantID,
						EventType: eventPlanExecutionStarted, BoundedContext: bcWorkflowEngine,
						Actor:      Actor{Kind: actorKindWorkflowEngine, ID: "wf-1", DisplayName: "Engine"},
						OccurredAt: now.Add(-time.Duration(i) * time.Second),
					})
				}
				return events, nil
			},
		},
		now: func() time.Time { return now },
	}

	resp, err := handler.ListAuditEvents(
		auditContext(tenantID),
		connect.NewRequest(&auditv1.ListAuditEventsRequest{
			TenantId: tenantID.String(),
			PageSize: 250, // above max → must be capped
		}),
	)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if capturedPageSize != maxPageSize {
		t.Fatalf("page size not capped: got %d, want %d", capturedPageSize, maxPageSize)
	}
	if len(resp.Msg.Events) != maxPageSize {
		t.Fatalf("events len = %d, want %d (page trimmed)", len(resp.Msg.Events), maxPageSize)
	}
	if resp.Msg.NextPageToken == nil || resp.Msg.GetNextPageToken() == "" {
		t.Fatal("expected next page token because another page exists")
	}
	if resp.Msg.Events[0].EventType != auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_EXECUTION_STARTED {
		t.Fatalf("event type mapping wrong: %v", resp.Msg.Events[0].EventType)
	}
}

func TestListAuditEventsNoNextTokenWhenExhausted(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{
		repo: &stubLister{
			listFn: func(context.Context, uuid.UUID, Filters, time.Time, uuid.UUID, int) ([]Event, error) {
				return []Event{{
					ID: uuid.New(), TenantID: tenantID, EventType: eventApprovalCreated,
					BoundedContext: bcHumanInteraction,
					Actor:          Actor{Kind: actorKindHuman, ID: "u-1"},
					OccurredAt:     time.Now().UTC(),
				}}, nil
			},
		},
		now: time.Now,
	}

	resp, err := handler.ListAuditEvents(
		auditContext(tenantID),
		connect.NewRequest(&auditv1.ListAuditEventsRequest{TenantId: tenantID.String()}),
	)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if resp.Msg.NextPageToken != nil {
		t.Fatalf("expected no next token, got %q", resp.Msg.GetNextPageToken())
	}
}

func TestListAuditEventsMapsRepositoryError(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{
		repo: &stubLister{
			listFn: func(context.Context, uuid.UUID, Filters, time.Time, uuid.UUID, int) ([]Event, error) {
				return nil, errors.New("boom")
			},
		},
		now: time.Now,
	}
	_, err := handler.ListAuditEvents(
		auditContext(tenantID),
		connect.NewRequest(&auditv1.ListAuditEventsRequest{TenantId: tenantID.String()}),
	)
	if got := connect.CodeOf(err); got != connect.CodeInternal {
		t.Fatalf("code = %v, want %v", got, connect.CodeInternal)
	}
}

func TestEventToProtoPreservesOptionals(t *testing.T) {
	t.Parallel()

	trace := "trace-abc"
	dec := auditv1.FeedbackDecision_FEEDBACK_DECISION_APPROVE
	agentType := "planner"
	ev := Event{
		ID: uuid.New(), TenantID: uuid.New(),
		EventType: eventApprovalDecided, BoundedContext: bcHumanInteraction,
		Actor:      Actor{Kind: actorKindAgent, ID: "a-1", DisplayName: "Planner", AgentType: "planner"},
		HasSubject: true,
		Subject:    Subject{Type: subjectApprovalRequest, ID: "ap-1"},
		Diff:       []DiffEntry{{Field: "decision", Before: "pending", After: "approve", HasBefore: true, HasAfter: true}},
		Decision:   decisionApprove,
		TraceID:    trace,
		OccurredAt: time.Now().UTC(),
	}
	pb := eventToProto(&ev)
	if pb.GetActor().GetAgentType() != agentType {
		t.Fatalf("agent type = %q, want %q", pb.GetActor().GetAgentType(), agentType)
	}
	if pb.Subject == nil || pb.Subject.SubjectType != auditv1.AuditSubjectType_AUDIT_SUBJECT_TYPE_APPROVAL_REQUEST {
		t.Fatalf("subject not mapped: %+v", pb.Subject)
	}
	if pb.GetTraceId() != trace {
		t.Fatalf("trace = %q, want %q", pb.GetTraceId(), trace)
	}
	if pb.GetDecision() != dec {
		t.Fatalf("decision = %v, want %v", pb.GetDecision(), dec)
	}
	if pb.OccurredAt.AsTime().IsZero() {
		t.Fatal("occurred_at must be set")
	}
	if len(pb.PayloadDiff) != 1 || pb.PayloadDiff[0].GetBefore() != "pending" {
		t.Fatalf("diff not mapped: %+v", pb.PayloadDiff)
	}
}
