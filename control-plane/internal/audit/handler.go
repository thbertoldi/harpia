package audit

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditv1 "github.com/harpia/control-plane/gen/harpia/audit/v1"
	"github.com/harpia/control-plane/internal/identity"
)

// Handler implements auditv1connect.AuditServiceHandler. It is read-only:
// ListAuditEvents only queries the tenant-scoped ledger.
type Handler struct {
	repo lister
	now  func() time.Time
}

// HandlerOptions configures the handler.
type HandlerOptions struct {
	Repo lister
	Now  func() time.Time
}

// NewHandler constructs the read-only audit handler.
func NewHandler(opts HandlerOptions) (*Handler, error) {
	if opts.Repo == nil {
		return nil, errors.New("audit: repository is required")
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Handler{repo: opts.Repo, now: opts.Now}, nil
}

// ListAuditEvents returns a newest-first, keyset-paged slice of the caller's
// audit events. The tenant is authorized through identity.RequireTenant and
// the database boundary re-checks it via forced RLS, so a request can never
// read another tenant's rows. Malformed or filter-mismatched page tokens are
// rejected with InvalidArgument.
func (h *Handler) ListAuditEvents(ctx context.Context, req *connect.Request[auditv1.ListAuditEventsRequest]) (*connect.Response[auditv1.ListAuditEventsResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}

	filters, err := filtersFromProto(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	pageSize := int(req.Msg.PageSize)
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	scope := scopeDigest(filters)
	var cursorOccurredAt time.Time
	var cursorID uuid.UUID
	if req.Msg.PageToken != nil {
		token := req.Msg.GetPageToken()
		occurredAt, id, err := decodeCursor(token, scope)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		cursorOccurredAt, cursorID = occurredAt, id
	}

	events, err := h.repo.List(ctx, tenantID, filters, cursorOccurredAt, cursorID, pageSize)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var nextToken string
	// The repository returns pageSize+1 rows when another page exists; trim
	// the lookahead row and mint the next token from the last returned row.
	if len(events) > pageSize {
		events = events[:pageSize]
		last := events[len(events)-1]
		nextToken = encodeCursor(last.OccurredAt, last.ID, scope)
	}

	pbEvents := make([]*auditv1.AuditEvent, 0, len(events))
	for i := range events {
		pbEvents = append(pbEvents, eventToProto(&events[i]))
	}

	resp := &auditv1.ListAuditEventsResponse{Events: pbEvents}
	if nextToken != "" {
		resp.NextPageToken = &nextToken
	}
	return connect.NewResponse(resp), nil
}

func eventToProto(event *Event) *auditv1.AuditEvent {
	pb := &auditv1.AuditEvent{
		EventId:        event.ID.String(),
		TenantId:       event.TenantID.String(),
		EventType:      eventTypeToProto(event.EventType),
		BoundedContext: boundedContextToProto(event.BoundedContext),
		Actor: &auditv1.AuditActor{
			Kind:        actorKindToProto(event.Actor.Kind),
			ActorId:     event.Actor.ID,
			DisplayName: event.Actor.DisplayName,
		},
		OccurredAt: timestamppb.New(event.OccurredAt),
	}
	if event.Actor.AgentType != "" {
		agentType := event.Actor.AgentType
		pb.Actor.AgentType = &agentType
	}
	if event.HasSubject {
		pb.Subject = &auditv1.AuditSubject{
			SubjectType: subjectTypeToProto(event.Subject.Type),
			SubjectId:   event.Subject.ID,
		}
	}
	if len(event.Diff) > 0 {
		pb.PayloadDiff = make([]*auditv1.PayloadDiffEntry, 0, len(event.Diff))
		for _, d := range event.Diff {
			entry := &auditv1.PayloadDiffEntry{Field: d.Field}
			if d.HasBefore {
				before := d.Before
				entry.Before = &before
			}
			if d.HasAfter {
				after := d.After
				entry.After = &after
			}
			pb.PayloadDiff = append(pb.PayloadDiff, entry)
		}
	}
	if event.TraceID != "" {
		traceID := event.TraceID
		pb.TraceId = &traceID
	}
	if event.Decision != "" {
		decision := decisionToProto(event.Decision)
		pb.Decision = &decision
	}
	return pb
}
