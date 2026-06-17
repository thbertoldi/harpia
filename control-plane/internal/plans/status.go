package plans

const (
	ConfigurationStatusDraft     = "draft"
	ConfigurationStatusRunnable  = "runnable"
	ConfigurationStatusScheduled = "scheduled"
	ConfigurationStatusDisabled  = "disabled"
	ConfigurationStatusArchived  = "archived"

	ExecutionStatusPending   = "pending"
	ExecutionStatusRunning   = "running"
	ExecutionStatusCompleted = "completed"
	ExecutionStatusFailed    = "failed"
	ExecutionStatusCancelled = "cancelled"

	StepStatusPending             = "pending"
	StepStatusRunning             = "running"
	StepStatusAwaitingElicitation = "awaiting_elicitation"
	StepStatusAwaitingApproval    = "awaiting_approval"
	StepStatusCompleted           = "completed"
	StepStatusFailed              = "failed"

	ApprovalRequestStatusPending  = "pending"
	ApprovalRequestStatusApproved = "approved"
	ApprovalRequestStatusRejected = "rejected"

	ElicitationStatusPending   = "pending"
	ElicitationStatusAnswered  = "answered"
	ElicitationStatusTimedOut  = "timed_out"
	ElicitationStatusCancelled = "cancelled"
)
