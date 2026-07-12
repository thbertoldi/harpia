package audit

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/harpia/control-plane/gen/harpia/executors/v1/executorsv1connect"
	"github.com/harpia/control-plane/gen/harpia/plans/v1/plansv1connect"
	"github.com/harpia/control-plane/internal/identity"
)

// Operation describes an existing public mutation that may be enriched by its
// application handler. Keeping this registry table-driven makes new audited
// operations an explicit review decision rather than an accidental side effect
// of a broad "all POSTs" rule.
type Operation struct {
	Procedure      string
	EventType      string
	BoundedContext string
	SubjectType    string
}

var existingOperations = []Operation{
	{plansv1connect.PlanServiceCreatePlanConfigurationProcedure, eventPlanConfigurationCreated, bcPlanManagement, subjectPlanConfiguration},
	{plansv1connect.PlanServiceUpdatePlanConfigurationProcedure, eventPlanConfigurationUpdated, bcPlanManagement, subjectPlanConfiguration},
	{plansv1connect.PlanServiceCreatePlanExecutionProcedure, eventPlanExecutionCreated, bcPlanManagement, subjectPlanExecution},
	{executorsv1connect.ExecutorServiceCreateExecutorInstallationProcedure, eventExecutorInstallationCreated, bcExecutorCatalog, subjectExecutorInstallation},
	{executorsv1connect.ExecutorServiceUpdateExecutorInstallationProcedure, eventExecutorInstallationUpdated, bcExecutorCatalog, subjectExecutorInstallation},
	{executorsv1connect.ExecutorServiceDeleteExecutorInstallationProcedure, eventExecutorInstallationDeleted, bcExecutorCatalog, subjectExecutorInstallation},
}

func operationForProcedure(procedure string) (Operation, bool) {
	for _, operation := range existingOperations {
		if operation.Procedure == procedure {
			return operation, true
		}
	}
	return Operation{}, false
}

type requestDraftKey struct{}

// requestDraft is deliberately shared through the request context. The audit
// interceptor creates it before invoking the handler; a successful handler can
// enrich it with its semantic subject and safe before/after values. Contexts
// are immutable, so a shared holder is required for the outer interceptor to
// read the enrichment after next returns.
type requestDraft struct {
	draft    EventDraft
	enriched bool
}

// EnrichRequestDraft attaches operation-specific, safe metadata for the audit
// interceptor. It is a no-op outside an audited public request and never writes
// to the recorder itself, preserving one event at the successful RPC boundary.
func EnrichRequestDraft(ctx context.Context, draft EventDraft) {
	holder, ok := ctx.Value(requestDraftKey{}).(*requestDraft)
	if !ok || holder == nil {
		return
	}
	holder.draft = draft
	holder.enriched = true
}

// AuditInterceptor records only after a unary request succeeds. It must sit
// after identity.NewRequestContextInterceptor so tenant and actor data are
// resolved before it evaluates the response boundary.
type AuditInterceptor struct {
	recorder Recorder
}

func NewAuditInterceptor(recorder Recorder) *AuditInterceptor {
	return &AuditInterceptor{recorder: recorder}
}

func (i *AuditInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		operation, targeted := operationForProcedure(req.Spec().Procedure)
		holder := &requestDraft{}
		if targeted {
			ctx = context.WithValue(ctx, requestDraftKey{}, holder)
		}

		response, err := next(ctx, req)
		if err != nil || i == nil || i.recorder == nil {
			return response, err
		}
		rc, ok := identity.RequestContextFrom(ctx)
		if !ok || rc.TenantID == uuid.Nil || strings.TrimSpace(rc.UserID) == "" {
			return response, err
		}

		// Dev authentication is a successful identity-resolution event, not a
		// login RPC. Exactly one is recorded for each successful enabled dev-token
		// request, independently of any mutation event that same request performs.
		if rc.UsedDevAuth {
			i.record(ctx, EventDraft{
				TenantID:       rc.TenantID,
				EventType:      eventAuthenticationDevAuth,
				BoundedContext: bcIdentityTenants,
				Actor:          humanActor(rc),
				TraceID:        traceID(req),
			})
		}

		if targeted && holder.enriched {
			draft := holder.draft
			// The handler is authoritative for semantic details, but identity and
			// trace data always come from the resolved transport boundary.
			draft.TenantID = rc.TenantID
			draft.Actor = humanActor(rc)
			if draft.TraceID == "" {
				draft.TraceID = traceID(req)
			}
			if draft.EventType == "" {
				draft.EventType = operation.EventType
			}
			if draft.BoundedContext == "" {
				draft.BoundedContext = operation.BoundedContext
			}
			if draft.HasSubject && draft.Subject.Type == "" {
				draft.Subject.Type = operation.SubjectType
			}
			i.record(ctx, draft)
		}
		return response, err
	}
}

func (i *AuditInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *AuditInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func (i *AuditInterceptor) record(ctx context.Context, draft EventDraft) {
	// Audit is best effort. A full queue or invalid future enrichment must not
	// turn an already-authorized operation into a failed response.
	_ = i.recorder.Record(ctx, draft)
}

func humanActor(rc identity.RequestContext) Actor {
	return Actor{Kind: actorKindHuman, ID: rc.UserID, DisplayName: "Human"}
}

func traceID(req connect.AnyRequest) string {
	if value := strings.TrimSpace(req.Header().Get("X-Trace-ID")); value != "" {
		return value
	}
	// Preserve a W3C traceparent only as correlation metadata; it never contains
	// an authorization credential. Prefer the explicit application trace header.
	return strings.TrimSpace(req.Header().Get("Traceparent"))
}
