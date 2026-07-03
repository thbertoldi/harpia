package workflow

const (
	// TaskQueueName is polled by the Go worker for plan workflows and integration/lifecycle activities.
	TaskQueueName = "harpia-task-queue"
	// AgentTaskQueueName is polled only by the Python agent-runtime worker, which owns RunAgentActivity.
	AgentTaskQueueName = "harpia-agent-task-queue"
)
