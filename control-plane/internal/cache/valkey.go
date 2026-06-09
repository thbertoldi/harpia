package cache

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/harpia/control-plane/internal/identity"
)

var ErrTenantBoundary = errors.New("cache key crosses tenant boundary")

type Client struct {
	rdb *redis.Client
}

type TenantStore struct {
	client *Client
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

func NewTenantStore(client *Client) *TenantStore {
	return &TenantStore{client: client}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (s *TenantStore) Get(ctx context.Context, key string) (string, error) {
	prefixed, err := s.tenantKey(ctx, key)
	if err != nil {
		return "", err
	}
	return s.client.rdb.Get(ctx, prefixed).Result()
}

func (s *TenantStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	prefixed, err := s.tenantKey(ctx, key)
	if err != nil {
		return err
	}
	return s.client.rdb.Set(ctx, prefixed, value, ttl).Err()
}

func (s *TenantStore) Delete(ctx context.Context, key string) error {
	prefixed, err := s.tenantKey(ctx, key)
	if err != nil {
		return err
	}
	return s.client.rdb.Del(ctx, prefixed).Err()
}

func (s *TenantStore) EvalInt(ctx context.Context, script string, keys []string, args ...any) (int, error) {
	prefixed, err := s.tenantKeys(ctx, keys)
	if err != nil {
		return 0, err
	}
	return s.client.rdb.Eval(ctx, script, prefixed, args...).Int()
}

func (s *TenantStore) tenantKeys(ctx context.Context, keys []string) ([]string, error) {
	prefixed := make([]string, 0, len(keys))
	for _, key := range keys {
		tenantKey, err := s.tenantKey(ctx, key)
		if err != nil {
			return nil, err
		}
		prefixed = append(prefixed, tenantKey)
	}
	return prefixed, nil
}

func (s *TenantStore) tenantKey(ctx context.Context, key string) (string, error) {
	if s == nil || s.client == nil {
		return "", errors.New("cache tenant store is not configured")
	}
	if strings.TrimSpace(key) == "" {
		return "", errors.New("cache key is required")
	}
	if strings.HasPrefix(key, "t:") {
		return "", fmt.Errorf("%w: %q is already tenant-scoped", ErrTenantBoundary, key)
	}

	tenantID, err := identity.RequireSelectedTenant(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("t:%s:%s", tenantID.String(), key), nil
}
