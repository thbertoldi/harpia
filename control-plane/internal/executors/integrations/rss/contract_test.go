package rss_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/executors/contracttest"
	"github.com/harpia/control-plane/internal/executors/integrations/rss"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

func TestRSSConfigValidatorRejectsInvalidFeeds(t *testing.T) {
	validator := rss.NewConfigValidator()
	if err := validator.ValidateConfig(json.RawMessage(`{"feeds":["not-a-url"]}`)); err == nil {
		t.Fatal("expected invalid feed URL error")
	}
}

func TestRSSIntegrationContract(t *testing.T) {
	store := contracttest.NewRSSArtifactStore()
	handler := rss.NewHandler(store, stubFeedFetcher{feed: &gofeed.Feed{
		Title: "Example News",
		Items: []*gofeed.Item{{
			Title:           "Story",
			Link:            "https://example.com/story",
			Description:     "Summary",
			PublishedParsed: timePtr(time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)),
		}},
	}})

	contracttest.Run(t, contracttest.Suite{
		Name:                    executors.SKURSSNewsFeed,
		Handler:                 handler,
		Store:                   store,
		SKUKey:                  executors.SKURSSNewsFeed,
		InputArtifactTypeKey:    artifacts.TypeKeyDateRange,
		OutputArtifactTypeKey:   artifacts.TypeKeyNewsList,
		ValidInstallationConfig: json.RawMessage(`{"feeds":["https://example.com/rss"]}`),
		ValidInputLiteralJSON:   contracttest.ValidRSSDateRangeLiteral,
		RetryableTrigger: func(_ context.Context, req runtime.IntegrationExecutionRequest) error {
			deadHandler := rss.NewHandler(store, stubFeedFetcher{err: errors.New("connection refused")})
			_, err := deadHandler.Execute(context.Background(), req)
			return err
		},
	})
}
