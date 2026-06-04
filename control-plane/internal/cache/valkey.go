package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(valkeyURL string) (*Client, error) {
	parsed, err := url.Parse(valkeyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Valkey URL: %w", err)
	}

	opts := &redis.Options{
		Network: "tcp",
	}

	if parsed.Host != "" {
		host, port := parsed.Hostname(), parsed.Port()
		if host == "" {
			host = "localhost"
		}
		if port == "" {
			port = "6379"
		}
		opts.Addr = fmt.Sprintf("%s:%s", host, port)
	} else {
		opts.Addr = "localhost:6379"
	}

	if parsed.User != nil {
		opts.Username = parsed.User.Username()
		if pass, ok := parsed.User.Password(); ok {
			opts.Password = pass
		}
	} else {
		password := parsed.Query().Get("password")
		if password != "" {
			opts.Password = password
		}
	}

	db := parsed.Query().Get("db")
	if db != "" {
		fmt.Sscanf(db, "%d", &opts.DB)
	}

	if strings.EqualFold(parsed.Scheme, "valkeys") || parsed.Query().Get("tls") == "true" {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	rdb := redis.NewClient(opts)

	return &Client{rdb: rdb}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

func (c *Client) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

func (c *Client) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}
