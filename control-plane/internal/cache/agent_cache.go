package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AgentCapabilityCache struct {
	client *Client
}

func NewAgentCapabilityCache(client *Client) *AgentCapabilityCache {
	return &AgentCapabilityCache{client: client}
}

func (c *AgentCapabilityCache) GetBestAgent(ctx context.Context, tenantID uuid.UUID, taskDescription string) (string, error) {
	key := capabilityKey(tenantID, taskDescription)
	return c.client.Get(ctx, key)
}

func (c *AgentCapabilityCache) SetBestAgent(ctx context.Context, tenantID uuid.UUID, taskDescription string, agentTypeID string) error {
	key := capabilityKey(tenantID, taskDescription)
	return c.client.Set(ctx, key, agentTypeID, 1*time.Hour)
}

func capabilityKey(tenantID uuid.UUID, taskDescription string) string {
	hash := sha256.Sum256([]byte(taskDescription))
	return fmt.Sprintf("capability:%s:%x", tenantID.String(), hash)
}
