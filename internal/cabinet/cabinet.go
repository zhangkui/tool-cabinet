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
	// A stoppable timer avoids leaking timers past the hold window when a
	// reservation is cancelled mid-wait (common under concurrent booking).
	timer := time.NewTimer(hold)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		// Restore the shared tool state so the cancellation does not poison
		// it for the next resident. The reservation never reached "borrowed",
		// so rolling back to "available" keeps concurrent bookers consistent.
		c.mu.Lock()
		c.tools[toolID] = "available"
		c.mu.Unlock()
		return ctx.Err()
	case <-timer.C:
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
