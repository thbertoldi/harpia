package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

type AgentCapabilityCache struct {
	client *Client
}

func NewAgentCapabilityCache(client *Client) *AgentCapabilityCache {
	return &AgentCapabilityCache{client: client}
}

func (c *AgentCapabilityCache) GetBestAgent(ctx context.Context, taskDescription string) (string, error) {
	key := capabilityKey(taskDescription)
	return c.client.Get(ctx, key)
}

func (c *AgentCapabilityCache) SetBestAgent(ctx context.Context, taskDescription string, agentTypeID string) error {
	key := capabilityKey(taskDescription)
	return c.client.Set(ctx, key, agentTypeID, 1*time.Hour)
}

func capabilityKey(taskDescription string) string {
	hash := sha256.Sum256([]byte(taskDescription))
	return fmt.Sprintf("capability:%x", hash)
}
