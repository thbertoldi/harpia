package linkedin

import "errors"

var (
	ErrInvalidConfig = errors.New("invalid linkedin publish configuration")
	ErrInvalidInput  = errors.New("invalid linkedin publish input")
)

// OAuthReconnectError indicates the tenant must reconnect LinkedIn credentials.
type OAuthReconnectError struct {
	Message string
	Cause   error
}

func (e *OAuthReconnectError) Error() string {
	if e == nil {
		return "linkedin oauth reconnect required"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "linkedin oauth reconnect required"
}

func (e *OAuthReconnectError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewOAuthReconnectError(message string, cause error) error {
	return &OAuthReconnectError{
		Message: message,
		Cause:   cause,
	}
}

func IsOAuthReconnectRequired(err error) bool {
	var reconnectErr *OAuthReconnectError
	return errors.As(err, &reconnectErr)
}

// PublishTransientError indicates a transient LinkedIn API failure.
type PublishTransientError struct {
	StatusCode int
	Cause      error
}

func (e *PublishTransientError) Error() string {
	if e == nil {
		return "linkedin publish transient failure"
	}
	if e.Cause == nil {
		return "linkedin publish transient failure"
	}
	return e.Cause.Error()
}

func (e *PublishTransientError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewPublishTransientError(statusCode int, cause error) error {
	return &PublishTransientError{
		StatusCode: statusCode,
		Cause:      cause,
	}
}
