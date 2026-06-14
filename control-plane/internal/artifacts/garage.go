package artifacts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/storage"
)

type PayloadStore interface {
	Put(ctx context.Context, objectPath string, payload []byte) (storageURI string, err error)
	Get(ctx context.Context, storageURI string) ([]byte, error)
}

type GarageStore struct {
	client     *s3.Client
	bucket     string
	pathPrefix *storage.TenantObjectStore
}

type GarageConfig struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
}

func NewGarageStore(cfg GarageConfig, pathPrefix *storage.TenantObjectStore) (*GarageStore, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("garage endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("garage bucket is required")
	}
	if pathPrefix == nil {
		return nil, fmt.Errorf("tenant object store is required")
	}

	endpointURL, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse garage endpoint: %w", err)
	}

	awsCfg := aws.Config{
		Region: cfg.Region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpointURL.String())
		o.UsePathStyle = true
	})

	return &GarageStore{
		client:     client,
		bucket:     cfg.Bucket,
		pathPrefix: pathPrefix,
	}, nil
}

func (s *GarageStore) Put(ctx context.Context, objectPath string, payload []byte) (string, error) {
	tenantPath, err := s.pathPrefix.ObjectPath(ctx, objectPath)
	if err != nil {
		return "", err
	}

	if err := s.pathPrefix.AssertTenantPath(ctx, tenantPath); err != nil {
		return "", err
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(tenantPath),
		Body:        bytes.NewReader(payload),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return "", fmt.Errorf("put garage object: %w", err)
	}

	return formatStorageURI(s.bucket, tenantPath), nil
}

func (s *GarageStore) Get(ctx context.Context, storageURI string) ([]byte, error) {
	bucket, key, err := parseStorageURI(storageURI)
	if err != nil {
		return nil, err
	}
	if bucket != s.bucket {
		return nil, fmt.Errorf("storage uri bucket %q does not match configured bucket %q", bucket, s.bucket)
	}

	if err := s.pathPrefix.AssertTenantPath(ctx, key); err != nil {
		return nil, err
	}

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get garage object: %w", err)
	}
	defer out.Body.Close()

	payload, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("read garage object: %w", err)
	}
	return payload, nil
}

func ContentHash(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func ArtifactObjectPath(artifactID uuid.UUID, stepExecutionID string) string {
	if stepExecutionID != "" {
		return path.Join("artifacts", "steps", stepExecutionID, artifactID.String()+".json")
	}
	return path.Join("artifacts", artifactID.String()+".json")
}

func formatStorageURI(bucket, key string) string {
	return "s3://" + bucket + "/" + key
}

func parseStorageURI(storageURI string) (bucket, key string, err error) {
	parsed, err := url.Parse(storageURI)
	if err != nil {
		return "", "", fmt.Errorf("parse storage uri: %w", err)
	}
	if parsed.Scheme != "s3" || parsed.Host == "" {
		return "", "", fmt.Errorf("invalid storage uri %q", storageURI)
	}
	key = strings.TrimPrefix(parsed.Path, "/")
	if key == "" {
		return "", "", fmt.Errorf("invalid storage uri %q", storageURI)
	}
	return parsed.Host, key, nil
}
