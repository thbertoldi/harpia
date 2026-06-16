package rss

import "errors"

var (
	ErrInvalidConfig = errors.New("invalid rss feed configuration")
	ErrInvalidInput  = errors.New("invalid fetch-news input")
)

// FeedFetchError indicates a transient feed fetch or parse failure that Temporal should retry.
type FeedFetchError struct {
	FeedURL string
	Cause   error
}

func (e *FeedFetchError) Error() string {
	if e == nil {
		return "feed fetch failed"
	}
	if e.FeedURL != "" {
		return "feed fetch failed for " + e.FeedURL + ": " + e.Cause.Error()
	}
	return "feed fetch failed: " + e.Cause.Error()
}

func (e *FeedFetchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewFeedFetchError(feedURL string, cause error) error {
	return &FeedFetchError{FeedURL: feedURL, Cause: cause}
}
