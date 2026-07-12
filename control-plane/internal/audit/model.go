// Package audit is the application/infrastructure adapter that persists
// governed operations to the append-only audit_events ledger and exposes
// them through the read-only AuditService.
//
// It is intentionally an adapter, not a domain package: it depends on
// Connect, pgx, and the generated protobuf bindings, while domain
// packages (plans, executors, workflow) depend only on the narrow
// Recorder port supplied by the recorder.go file. This keeps the
// tenant-safe database boundary centralized and prevents database/transport
// concerns from leaking into business domains.
package audit

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	auditv1 "github.com/harpia/control-plane/gen/harpia/audit/v1"
)

// Domain vocabularies. These are the short, stable strings persisted in
// audit_events. They are application-validated (the proto enum is the
// contract surface); the DB stores TEXT so the ledger does not require a
// migration whenever the vocabulary grows.
const (
	actorKindHuman          = "human"
	actorKindAgent          = "agent"
	actorKindWorkflowEngine = "workflow_engine"

	bcPlanManagement   = "plan_management"
	bcHumanInteraction = "human_interaction"
	bcIdentityTenants  = "identity_tenants"
	bcWorkflowEngine   = "workflow_engine"
	bcExecutorCatalog  = "executor_catalog"

	subjectPlanConfiguration    = "plan_configuration"
	subjectPlanExecution        = "plan_execution"
	subjectStepExecution        = "step_execution"
	subjectExecutorInstallation = "executor_installation"
	subjectApprovalRequest      = "approval_request"

	decisionApprove  = "approve"
	decisionReject   = "reject"
	decisionModify   = "modify"
	decisionEscalate = "escalate"

	// defaultPageSize is the page size applied when a caller omits one.
	defaultPageSize = 50
	// maxPageSize caps the page size accepted by the public handler.
	maxPageSize = 200

	// event type strings — kept in sync with the proto AuditEventType enum.
	eventPlanConfigurationCreated       = "plan_configuration.created"
	eventPlanConfigurationUpdated       = "plan_configuration.updated"
	eventPlanConfigurationStatusChanged = "plan_configuration.status_changed"
	eventPlanExecutionCreated           = "plan_execution.created"
	eventPlanExecutionStarted           = "plan_execution.started"
	eventPlanExecutionCompleted         = "plan_execution.completed"
	eventPlanExecutionFailed            = "plan_execution.failed"
	eventStepExecutionStarted           = "step_execution.started"
	eventStepExecutionCompleted         = "step_execution.completed"
	eventStepExecutionFailed            = "step_execution.failed"
	eventStepExecutionSkipped           = "step_execution.skipped"
	eventApprovalCreated                = "approval.created"
	eventApprovalDecided                = "approval.decided"
	eventWorkflowSignalReceived         = "workflow.signal_received"
	eventAuthenticationDevAuth          = "authentication.dev_auth"
	eventExecutorInstallationCreated    = "executor_installation.created"
	eventExecutorInstallationUpdated    = "executor_installation.updated"
	eventExecutorInstallationDeleted    = "executor_installation.deleted"

	// proto-enum sentinels used to validate persisted vocabularies. They
	// mirror the zero values of the generated enums.
	auditEventTypeUnspecified   = 0
	boundedContextUnspecified   = 0
	feedbackDecisionUnspecified = 0
)

// Event is the internal representation of one audited transition.
type Event struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	DedupeKey      string // optional idempotency key; "" means no dedupe
	EventType      string
	BoundedContext string
	Actor          Actor
	Subject        Subject // HasSubject=false for authentication events
	HasSubject     bool
	Diff           []DiffEntry
	Decision       string // "" when not applicable
	TraceID        string
	OccurredAt     time.Time
}

// Actor is a snapshot of the operation initiator.
type Actor struct {
	Kind        string
	ID          string
	DisplayName string
	AgentType   string // non-empty only for agent/workflow actors
}

// Subject is the typed entity the event concerns.
type Subject struct {
	Type string
	ID   string
}

// DiffEntry captures one safe before/after field transition.
type DiffEntry struct {
	Field  string
	Before string // "" represents null/absent
	After  string
	// HasBefore/HasAfter distinguish an explicit empty string from a null
	// value, so the proto optional mapping is faithful.
	HasBefore bool
	HasAfter  bool
}

// Filters describes the active listing predicates, normalized for both
// the repository query and the cursor scope binding.
type Filters struct {
	EventTypes []string
	Actor      *ActorFilter
	DateFrom   time.Time // zero = unbounded
	DateTo     time.Time // zero = unbounded
	SubjectID  string
	Decision   string
}

// ActorFilter narrows a listing to one actor.
type ActorFilter struct {
	Kind      string
	ID        string
	AgentType string
}

// ErrInvalidCursor is returned when an opaque page token is malformed or
// was minted for a different filter scope.
var ErrInvalidCursor = errors.New("audit: invalid or mismatched cursor")

// actorKindToProto maps the persisted actor kind to its proto enum.
func actorKindToProto(kind string) auditv1.ActorKind {
	switch kind {
	case actorKindHuman:
		return auditv1.ActorKind_ACTOR_KIND_HUMAN
	case actorKindAgent:
		return auditv1.ActorKind_ACTOR_KIND_AGENT
	case actorKindWorkflowEngine:
		return auditv1.ActorKind_ACTOR_KIND_WORKFLOW_ENGINE
	default:
		return auditv1.ActorKind_ACTOR_KIND_UNSPECIFIED
	}
}

func actorKindFromProto(kind auditv1.ActorKind) (string, error) {
	switch kind {
	case auditv1.ActorKind_ACTOR_KIND_HUMAN:
		return actorKindHuman, nil
	case auditv1.ActorKind_ACTOR_KIND_AGENT:
		return actorKindAgent, nil
	case auditv1.ActorKind_ACTOR_KIND_WORKFLOW_ENGINE:
		return actorKindWorkflowEngine, nil
	default:
		return "", errors.New("actor kind is required")
	}
}

func boundedContextToProto(bc string) auditv1.BoundedContext {
	switch bc {
	case bcPlanManagement:
		return auditv1.BoundedContext_BOUNDED_CONTEXT_PLAN_MANAGEMENT
	case bcHumanInteraction:
		return auditv1.BoundedContext_BOUNDED_CONTEXT_HUMAN_INTERACTION
	case bcIdentityTenants:
		return auditv1.BoundedContext_BOUNDED_CONTEXT_IDENTITY_TENANTS
	case bcWorkflowEngine:
		return auditv1.BoundedContext_BOUNDED_CONTEXT_WORKFLOW_ENGINE
	case bcExecutorCatalog:
		return auditv1.BoundedContext_BOUNDED_CONTEXT_EXECUTOR_CATALOG
	default:
		return auditv1.BoundedContext_BOUNDED_CONTEXT_UNSPECIFIED
	}
}

func subjectTypeToProto(t string) auditv1.AuditSubjectType {
	switch t {
	case subjectPlanConfiguration:
		return auditv1.AuditSubjectType_AUDIT_SUBJECT_TYPE_PLAN_CONFIGURATION
	case subjectPlanExecution:
		return auditv1.AuditSubjectType_AUDIT_SUBJECT_TYPE_PLAN_EXECUTION
	case subjectStepExecution:
		return auditv1.AuditSubjectType_AUDIT_SUBJECT_TYPE_STEP_EXECUTION
	case subjectExecutorInstallation:
		return auditv1.AuditSubjectType_AUDIT_SUBJECT_TYPE_EXECUTOR_INSTALLATION
	case subjectApprovalRequest:
		return auditv1.AuditSubjectType_AUDIT_SUBJECT_TYPE_APPROVAL_REQUEST
	default:
		return auditv1.AuditSubjectType_AUDIT_SUBJECT_TYPE_UNSPECIFIED
	}
}

func decisionToProto(d string) auditv1.FeedbackDecision {
	switch d {
	case decisionApprove:
		return auditv1.FeedbackDecision_FEEDBACK_DECISION_APPROVE
	case decisionReject:
		return auditv1.FeedbackDecision_FEEDBACK_DECISION_REJECT
	case decisionModify:
		return auditv1.FeedbackDecision_FEEDBACK_DECISION_MODIFY
	case decisionEscalate:
		return auditv1.FeedbackDecision_FEEDBACK_DECISION_ESCALATE
	default:
		return auditv1.FeedbackDecision_FEEDBACK_DECISION_UNSPECIFIED
	}
}

func decisionFromProto(d auditv1.FeedbackDecision) (string, error) {
	switch d {
	case auditv1.FeedbackDecision_FEEDBACK_DECISION_APPROVE:
		return decisionApprove, nil
	case auditv1.FeedbackDecision_FEEDBACK_DECISION_REJECT:
		return decisionReject, nil
	case auditv1.FeedbackDecision_FEEDBACK_DECISION_MODIFY:
		return decisionModify, nil
	case auditv1.FeedbackDecision_FEEDBACK_DECISION_ESCALATE:
		return decisionEscalate, nil
	default:
		return "", errors.New("decision must be a known feedback value")
	}
}

func eventTypeToProto(t string) auditv1.AuditEventType {
	switch t {
	case eventPlanConfigurationCreated:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_CONFIGURATION_CREATED
	case eventPlanConfigurationUpdated:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_CONFIGURATION_UPDATED
	case eventPlanConfigurationStatusChanged:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_CONFIGURATION_STATUS_CHANGED
	case eventPlanExecutionCreated:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_EXECUTION_CREATED
	case eventPlanExecutionStarted:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_EXECUTION_STARTED
	case eventPlanExecutionCompleted:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_EXECUTION_COMPLETED
	case eventPlanExecutionFailed:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_EXECUTION_FAILED
	case eventStepExecutionStarted:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_STEP_EXECUTION_STARTED
	case eventStepExecutionCompleted:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_STEP_EXECUTION_COMPLETED
	case eventStepExecutionFailed:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_STEP_EXECUTION_FAILED
	case eventStepExecutionSkipped:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_STEP_EXECUTION_SKIPPED
	case eventApprovalCreated:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_APPROVAL_CREATED
	case eventApprovalDecided:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_APPROVAL_DECIDED
	case eventWorkflowSignalReceived:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_WORKFLOW_SIGNAL_RECEIVED
	case eventAuthenticationDevAuth:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_AUTHENTICATION_DEV_AUTH
	case eventExecutorInstallationCreated:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_EXECUTOR_INSTALLATION_CREATED
	case eventExecutorInstallationUpdated:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_EXECUTOR_INSTALLATION_UPDATED
	case eventExecutorInstallationDeleted:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_EXECUTOR_INSTALLATION_DELETED
	default:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_UNSPECIFIED
	}
}

func eventTypeFromProto(t auditv1.AuditEventType) (string, error) {
	switch t {
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_CONFIGURATION_CREATED:
		return eventPlanConfigurationCreated, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_CONFIGURATION_UPDATED:
		return eventPlanConfigurationUpdated, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_CONFIGURATION_STATUS_CHANGED:
		return eventPlanConfigurationStatusChanged, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_EXECUTION_CREATED:
		return eventPlanExecutionCreated, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_EXECUTION_STARTED:
		return eventPlanExecutionStarted, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_EXECUTION_COMPLETED:
		return eventPlanExecutionCompleted, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_PLAN_EXECUTION_FAILED:
		return eventPlanExecutionFailed, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_STEP_EXECUTION_STARTED:
		return eventStepExecutionStarted, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_STEP_EXECUTION_COMPLETED:
		return eventStepExecutionCompleted, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_STEP_EXECUTION_FAILED:
		return eventStepExecutionFailed, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_STEP_EXECUTION_SKIPPED:
		return eventStepExecutionSkipped, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_APPROVAL_CREATED:
		return eventApprovalCreated, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_APPROVAL_DECIDED:
		return eventApprovalDecided, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_WORKFLOW_SIGNAL_RECEIVED:
		return eventWorkflowSignalReceived, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_AUTHENTICATION_DEV_AUTH:
		return eventAuthenticationDevAuth, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_EXECUTOR_INSTALLATION_CREATED:
		return eventExecutorInstallationCreated, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_EXECUTOR_INSTALLATION_UPDATED:
		return eventExecutorInstallationUpdated, nil
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_EXECUTOR_INSTALLATION_DELETED:
		return eventExecutorInstallationDeleted, nil
	default:
		return "", fmt.Errorf("audit: unsupported event type %v", t)
	}
}

// filtersFromProto normalizes the request filters into the Filters struct
// used by both the repository and the cursor scope. It validates enums and
// date bounds; an error yields InvalidArgument at the handler boundary.
func filtersFromProto(req *auditv1.ListAuditEventsRequest) (Filters, error) {
	filters := Filters{}

	if len(req.EventTypes) > 0 {
		seen := make(map[string]struct{}, len(req.EventTypes))
		for _, et := range req.EventTypes {
			s, err := eventTypeFromProto(et)
			if err != nil {
				return Filters{}, err
			}
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			filters.EventTypes = append(filters.EventTypes, s)
		}
	}

	if req.Actor != nil {
		kind, err := actorKindFromProto(req.Actor.Kind)
		if err != nil {
			return Filters{}, err
		}
		if strings.TrimSpace(req.Actor.ActorId) == "" {
			return Filters{}, errors.New("actor filter requires actor_id")
		}
		filters.Actor = &ActorFilter{
			Kind:      kind,
			ID:        req.Actor.ActorId,
			AgentType: req.Actor.GetAgentType(),
		}
	}

	if req.DateFrom != nil {
		from, err := parseDateBound(req.GetDateFrom(), true)
		if err != nil {
			return Filters{}, err
		}
		filters.DateFrom = from
	}
	if req.DateTo != nil {
		to, err := parseDateBound(req.GetDateTo(), false)
		if err != nil {
			return Filters{}, err
		}
		filters.DateTo = to
	}
	if filters.DateFrom.IsZero() && !filters.DateTo.IsZero() {
		return Filters{}, errors.New("date_to requires date_from")
	}
	if !filters.DateFrom.IsZero() && !filters.DateTo.IsZero() && filters.DateFrom.After(filters.DateTo) {
		return Filters{}, errors.New("date_from must not be after date_to")
	}

	if req.SubjectId != nil {
		filters.SubjectID = strings.TrimSpace(req.GetSubjectId())
	}
	if req.Decision != nil {
		d, err := decisionFromProto(req.GetDecision())
		if err != nil {
			return Filters{}, err
		}
		filters.Decision = d
	}

	return filters, nil
}

// parseDateBound parses an inclusive ISO-8601 bound. A date-only value for
// date_from is widened to its start-of-day; date_to to end-of-day so the
// bound stays inclusive for date-range filters.
func parseDateBound(value string, lower bool) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("date bound is empty")
	}
	dateOnly := len(value) == 10 // YYYY-MM-DD
	if dateOnly {
		value = value + "T00:00:00Z"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("date bound must be ISO-8601 (RFC3339 or YYYY-MM-DD): %w", err)
	}
	if !lower && dateOnly {
		t = t.Add(24*time.Hour - time.Nanosecond)
	}
	return t.UTC(), nil
}
