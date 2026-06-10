package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

type AgentCapabilityCache struct {
	store *TenantStore
}

func NewAgentCapabilityCache(store *TenantStore) *AgentCapabilityCache {
	return &AgentCapabilityCache{store: store}
}

func (c *AgentCapabilityCache) GetBestAgent(ctx context.Context, taskDescription string) (string, error) {
	key := capabilityKey(taskDescription)
	return c.store.Get(ctx, key)
}

func (c *AgentCapabilityCache) SetBestAgent(ctx context.Context, taskDescription string, agentTypeID string) error {
	key := capabilityKey(taskDescription)
	return c.store.Set(ctx, key, agentTypeID, 1*time.Hour)
}

func capabilityKey(taskDescription string) string {
	hash := sha256.Sum256([]byte(taskDescription))
	return fmt.Sprintf("capability:%x", hash)
}
