package storage

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/harpia/control-plane/internal/identity"
)

const TenantObjectRoot = "tenant"

var ErrTenantBoundary = errors.New("object path crosses tenant boundary")

type TenantObjectStore struct {
	bucket string
}

func NewTenantObjectStore(bucket string) *TenantObjectStore {
	return &TenantObjectStore{bucket: bucket}
}

func (s *TenantObjectStore) Bucket() string {
	if s == nil {
		return ""
	}
	return s.bucket
}

func (s *TenantObjectStore) ObjectPath(ctx context.Context, logicalPath string) (string, error) {
	tenantID, err := identity.RequireSelectedTenant(ctx)
	if err != nil {
		return "", err
	}

	clean, err := cleanLogicalPath(logicalPath)
	if err != nil {
		return "", err
	}

	return path.Join(TenantObjectRoot, tenantID.String(), clean), nil
}

func (s *TenantObjectStore) AssertTenantPath(ctx context.Context, objectPath string) error {
	tenantID, err := identity.RequireSelectedTenant(ctx)
	if err != nil {
		return err
	}

	clean := path.Clean(strings.TrimLeft(strings.TrimSpace(objectPath), "/"))
	wantPrefix := path.Join(TenantObjectRoot, tenantID.String()) + "/"
	if !strings.HasPrefix(clean, wantPrefix) {
		return fmt.Errorf("%w: %q is outside %q", ErrTenantBoundary, objectPath, wantPrefix)
	}
	return nil
}

func cleanLogicalPath(logicalPath string) (string, error) {
	trimmed := strings.TrimSpace(logicalPath)
	if trimmed == "" {
		return "", errors.New("object path is required")
	}

	clean := path.Clean(strings.TrimLeft(trimmed, "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid object path %q", logicalPath)
	}
	if clean == TenantObjectRoot || strings.HasPrefix(clean, TenantObjectRoot+"/") {
		return "", fmt.Errorf("%w: %q is already tenant-scoped", ErrTenantBoundary, logicalPath)
	}
	return clean, nil
}
