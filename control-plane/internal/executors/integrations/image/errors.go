package image

import "errors"

var (
	ErrInvalidConfig = errors.New("invalid image generator configuration")
	ErrInvalidInput  = errors.New("invalid image generation input")
)

// ProviderError indicates a transient image provider API failure (rate limit,
// upstream outage, network error) that Temporal should retry. Non-transient
// provider failures (bad request, auth) should NOT be wrapped in ProviderError.
type ProviderError struct {
	Provider string
	Cause    error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "image provider failure"
	}
	if e.Cause == nil {
		return "image provider " + e.Provider + " failed"
	}
	return "image provider " + e.Provider + " failed: " + e.Cause.Error()
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewProviderError(provider string, cause error) error {
	return &ProviderError{Provider: provider, Cause: cause}
}
