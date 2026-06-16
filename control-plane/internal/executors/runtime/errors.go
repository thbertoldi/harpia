package runtime

import "errors"

// Retryable Temporal activity error codes. Integration handlers must use these
// constants so workflow adapters can map failures consistently.
const (
	ErrCodeFeedFetch              = "FeedFetchError"
	ErrCodeLinkedInPublish        = "LinkedInPublishError"
	ErrCodeOAuthReconnectRequired = "OAuthReconnectRequired"
)

// RetryableError marks a transient integration failure that Temporal should retry.
// Return this error from IntegrationHandler.Execute; do not swallow it as a failed result.
type RetryableError struct {
	Code    string
	Message string
	Cause   error
}

func NewRetryableError(code, message string, cause error) *RetryableError {
	return &RetryableError{Code: code, Message: message, Cause: cause}
}

func (e *RetryableError) Error() string {
	if e == nil {
		return "retryable integration error"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "retryable integration error"
}

func (e *RetryableError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// TerminalValidationError marks non-retryable configuration or input validation failures.
type TerminalValidationError struct {
	Message string
	Cause   error
}

func (e *TerminalValidationError) Error() string {
	if e == nil {
		return "terminal validation error"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "terminal validation error"
}

func (e *TerminalValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func IsRetryable(err error) (*RetryableError, bool) {
	var retryable *RetryableError
	if errors.As(err, &retryable) {
		return retryable, true
	}
	return nil, false
}

func failedIntegrationResult(message string) IntegrationExecutionResult {
	return IntegrationExecutionResult{
		Status: IntegrationStatusFailed,
		Error:  message,
	}
}
