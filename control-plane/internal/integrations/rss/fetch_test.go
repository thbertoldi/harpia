package rss

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
)

const sampleFeedXML = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Example News</title>
    <item>
      <title>In-range story</title>
      <link>https://example.com/in-range</link>
      <description>Inside the configured date range.</description>
      <pubDate>Mon, 05 Jan 2026 12:00:00 GMT</pubDate>
    </item>
    <item>
      <title>Out-of-range story</title>
      <link>https://example.com/out-of-range</link>
      <description>Outside the configured date range.</description>
      <pubDate>Mon, 01 Dec 2025 12:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

func TestFetchNewsListSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleFeedXML))
	}))
	defer server.Close()

	fetcher := NewHTTPFeedFetcher(server.Client())
	newsList, err := FetchNewsList(context.Background(), fetcher, []string{server.URL}, &artifactsv1.DateRange{
		StartDate: "2026-01-01",
		EndDate:   "2026-01-07",
	})
	if err != nil {
		t.Fatalf("FetchNewsList() error = %v", err)
	}
	if len(newsList.Articles) != 1 {
		t.Fatalf("Articles = %d, want 1", len(newsList.Articles))
	}
	if newsList.Articles[0].Title != "In-range story" {
		t.Fatalf("Article title = %q", newsList.Articles[0].Title)
	}
	if newsList.Articles[0].Source != "Example News" {
		t.Fatalf("Article source = %q", newsList.Articles[0].Source)
	}
}

func TestFetchNewsListDeadFeedIsRetryable(t *testing.T) {
	fetcher := NewHTTPFeedFetcher(http.DefaultClient)
	_, err := FetchNewsList(context.Background(), fetcher, []string{"http://127.0.0.1:1/dead-feed"}, &artifactsv1.DateRange{
		StartDate: "2026-01-01",
		EndDate:   "2026-01-07",
	})
	if err == nil {
		t.Fatal("expected feed fetch error")
	}
	var feedErr *FeedFetchError
	if !errors.As(err, &feedErr) {
		t.Fatalf("error = %T(%v), want *FeedFetchError", err, err)
	}
}

func TestFetchNewsListParseFailureIsRetryable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not a feed</html>"))
	}))
	defer server.Close()

	fetcher := NewHTTPFeedFetcher(server.Client())
	_, err := FetchNewsList(context.Background(), fetcher, []string{server.URL}, &artifactsv1.DateRange{
		StartDate: "2026-01-01",
		EndDate:   "2026-01-07",
	})
	if err == nil {
		t.Fatal("expected feed fetch error")
	}
	var feedErr *FeedFetchError
	if !errors.As(err, &feedErr) {
		t.Fatalf("error = %T(%v), want *FeedFetchError", err, err)
	}
}

func TestFetchNewsListRejectsInvalidDateRange(t *testing.T) {
	fetcher := stubFeedFetcher{feed: &gofeed.Feed{Title: "Example News"}}
	_, err := FetchNewsList(context.Background(), fetcher, []string{"https://example.com/rss"}, &artifactsv1.DateRange{
		StartDate: "2026-01-07",
		EndDate:   "2026-01-01",
	})
	if err == nil {
		t.Fatal("expected invalid input error")
	}
}

type stubFeedFetcher struct {
	feed *gofeed.Feed
	err  error
}

func (s stubFeedFetcher) Fetch(_ context.Context, _ string) (*gofeed.Feed, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.feed, nil
}

func TestSupportsFetchNewsStep(t *testing.T) {
	if !SupportsFetchNewsStep("fetch-news", "") {
		t.Fatal("expected fetch-news step to be supported")
	}
	if !SupportsFetchNewsStep("", "rss-news-feed") {
		t.Fatal("expected rss-news-feed sku to be supported")
	}
	if SupportsFetchNewsStep("publish-linkedin", "linkedin-publish") {
		t.Fatal("did not expect linkedin step to be supported")
	}
}

func TestItemToNewsArticleUsesPublishedDate(t *testing.T) {
	published := time.Date(2026, 1, 5, 15, 4, 5, 0, time.UTC)
	article := itemToNewsArticle(&gofeed.Item{
		Title:           "Story",
		Link:            "https://example.com/story",
		Description:     strings.TrimSpace("Summary"),
		PublishedParsed: &published,
	}, "Example News")
	if article == nil {
		t.Fatal("expected article")
	}
	if article.PublishedAt == "" {
		t.Fatal("expected published_at to be set")
	}
}
