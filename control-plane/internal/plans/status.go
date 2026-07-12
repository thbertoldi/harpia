package plans

const (
	ConfigurationStatusDraft     = "draft"
	ConfigurationStatusRunnable  = "runnable"
	ConfigurationStatusScheduled = "scheduled"
	ConfigurationStatusDisabled  = "disabled"
	ConfigurationStatusArchived  = "archived"
	ConfigurationKindOneShot     = "one_shot"
	ConfigurationKindRecurring   = "recurring"

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
	StepStatusSkipped             = "skipped"

	ApprovalRequestStatusPending  = "pending"
	ApprovalRequestStatusApproved = "approved"
	ApprovalRequestStatusRejected = "rejected"

	ReviewRequestStatusPending           = "pending"
	ReviewRequestStatusAccepted          = "accepted"
	ReviewRequestStatusRevisionRequested = "revision_requested"
	ReviewRequestStatusCancelled         = "cancelled"

	ElicitationStatusPending   = "pending"
	ElicitationStatusAnswered  = "answered"
	ElicitationStatusTimedOut  = "timed_out"
	ElicitationStatusCancelled = "cancelled"
)
