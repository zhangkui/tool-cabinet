package cabinet

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrUnavailable = errors.New("tool unavailable")

type Cabinet struct {
	mu    sync.Mutex
	tools map[string]string
}

func New() *Cabinet {
	return &Cabinet{tools: map[string]string{"drill-01": "available", "ladder-01": "available"}}
}
func (c *Cabinet) Reserve(ctx context.Context, toolID string, hold time.Duration) error {
	c.mu.Lock()
	if c.tools[toolID] != "available" {
		c.mu.Unlock()
		return ErrUnavailable
	}
	c.tools[toolID] = "reserved"
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		c.mu.Lock()
		c.tools[toolID] = "available"
		c.mu.Unlock()
		return ctx.Err()
	case <-time.After(hold):
	}
	c.mu.Lock()
	c.tools[toolID] = "borrowed"
	c.mu.Unlock()
	return nil
}
func (c *Cabinet) Release(toolID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.tools[toolID]
	if !ok {
		return errors.New("tool not found")
	}
	if state != "borrowed" && state != "reserved" {
		return errors.New("tool is not borrowed")
	}
	c.tools[toolID] = "available"
	return nil
}
func (c *Cabinet) Status(toolID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.tools[toolID]
	return state, ok
}
