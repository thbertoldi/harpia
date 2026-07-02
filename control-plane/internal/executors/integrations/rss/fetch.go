package rss

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type FeedFetcher interface {
	Fetch(ctx context.Context, feedURL string) (*gofeed.Feed, error)
}

type HTTPFeedFetcher struct {
	client *http.Client
}

func NewHTTPFeedFetcher(client *http.Client) *HTTPFeedFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPFeedFetcher{client: client}
}

func (f *HTTPFeedFetcher) Fetch(ctx context.Context, feedURL string) (*gofeed.Feed, error) {
	parser := gofeed.NewParser()
	parser.Client = f.client
	feed, err := parser.ParseURLWithContext(feedURL, ctx)
	if err != nil {
		return nil, NewFeedFetchError(feedURL, err)
	}
	if feed == nil {
		return nil, NewFeedFetchError(feedURL, fmt.Errorf("empty feed response"))
	}
	return feed, nil
}

func FetchNewsList(ctx context.Context, fetcher FeedFetcher, feeds []string, dateRange *artifactsv1.DateRange) (*artifactsv1.NewsList, error) {
	if fetcher == nil {
		return nil, fmt.Errorf("%w: feed fetcher is required", ErrInvalidInput)
	}
	if dateRange == nil {
		return nil, fmt.Errorf("%w: date range is required", ErrInvalidInput)
	}

	startDate, endDate, err := parseDateRange(dateRange)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return fetchNewsListInRange(ctx, fetcher, feeds, startDate, endDate)
}

func fetchNewsListInRange(ctx context.Context, fetcher FeedFetcher, feeds []string, startDate, endDate time.Time) (*artifactsv1.NewsList, error) {
	if fetcher == nil {
		return nil, fmt.Errorf("%w: feed fetcher is required", ErrInvalidInput)
	}

	articlesByURL := make(map[string]*artifactsv1.NewsArticle)
	for _, feedURL := range feeds {
		feed, err := fetcher.Fetch(ctx, feedURL)
		if err != nil {
			return nil, err
		}
		source := strings.TrimSpace(feed.Title)
		if source == "" {
			source = feedURL
		}
		for _, item := range feed.Items {
			article := itemToNewsArticle(item, source)
			if article == nil {
				continue
			}
			if !articleInDateRange(article, startDate, endDate) {
				continue
			}
			if existing, ok := articlesByURL[article.Url]; ok {
				if articlePublishedAfter(article, existing) {
					articlesByURL[article.Url] = article
				}
				continue
			}
			articlesByURL[article.Url] = article
		}
	}

	if len(articlesByURL) == 0 {
		return nil, fmt.Errorf("%w: no articles found in date range", ErrInvalidInput)
	}

	articles := make([]*artifactsv1.NewsArticle, 0, len(articlesByURL))
	for _, article := range articlesByURL {
		articles = append(articles, article)
	}
	sort.Slice(articles, func(i, j int) bool {
		return articles[i].PublishedAt > articles[j].PublishedAt
	})

	return &artifactsv1.NewsList{Articles: articles}, nil
}

func itemToNewsArticle(item *gofeed.Item, source string) *artifactsv1.NewsArticle {
	if item == nil {
		return nil
	}
	title := strings.TrimSpace(item.Title)
	link := strings.TrimSpace(item.Link)
	if title == "" || link == "" {
		return nil
	}

	summary := strings.TrimSpace(item.Description)
	if summary == "" {
		summary = strings.TrimSpace(item.Content)
	}

	publishedAt := ""
	if item.PublishedParsed != nil {
		publishedAt = item.PublishedParsed.UTC().Format(time.RFC3339)
	} else if item.UpdatedParsed != nil {
		publishedAt = item.UpdatedParsed.UTC().Format(time.RFC3339)
	}

	return &artifactsv1.NewsArticle{
		Title:       title,
		Url:         link,
		Summary:     summary,
		Source:      source,
		PublishedAt: publishedAt,
	}
}

func parseDateRange(dateRange *artifactsv1.DateRange) (time.Time, time.Time, error) {
	startRaw := strings.TrimSpace(dateRange.StartDate)
	endRaw := strings.TrimSpace(dateRange.EndDate)
	if startRaw == "" || endRaw == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date and end_date are required")
	}

	startDate, err := time.Parse("2006-01-02", startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse start_date: %w", err)
	}
	endDate, err := time.Parse("2006-01-02", endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse end_date: %w", err)
	}
	if endDate.Before(startDate) {
		return time.Time{}, time.Time{}, fmt.Errorf("end_date must be on or after start_date")
	}

	startDate = startDate.UTC()
	endDate = endDate.UTC()
	return startDate, endDate, nil
}

func parseDateRangePayload(payload []byte) (time.Time, time.Time, error) {
	var raw struct {
		Preset string `json:"preset"`
	}
	if err := json.Unmarshal(payload, &raw); err == nil && strings.TrimSpace(raw.Preset) != "" {
		return time.Time{}, time.Time{}, fmt.Errorf("unresolved date range preset %q reached RSS executor", raw.Preset)
	}
	dateRange := &artifactsv1.DateRange{}
	if err := protojson.Unmarshal(payload, dateRange); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse date range artifact: %w", err)
	}
	return parseDateRange(dateRange)
}

func articleInDateRange(article *artifactsv1.NewsArticle, startDate, endDate time.Time) bool {
	if article == nil || strings.TrimSpace(article.PublishedAt) == "" {
		return false
	}
	published, err := time.Parse(time.RFC3339, article.PublishedAt)
	if err != nil {
		return false
	}
	publishedDate := published.UTC().Truncate(24 * time.Hour)
	return !publishedDate.Before(startDate) && !publishedDate.After(endDate)
}

func articlePublishedAfter(left, right *artifactsv1.NewsArticle) bool {
	leftTime, leftErr := time.Parse(time.RFC3339, left.GetPublishedAt())
	rightTime, rightErr := time.Parse(time.RFC3339, right.GetPublishedAt())
	if leftErr != nil {
		return false
	}
	if rightErr != nil {
		return true
	}
	return leftTime.After(rightTime)
}
