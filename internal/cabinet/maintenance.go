package cabinet

import (
	"errors"
	"time"
)

var ErrMaintenance = errors.New("tool under maintenance")

func (c *Cabinet) StartMaintenance(toolID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.tools[toolID]; !ok {
		return errors.New("tool not found")
	}
	c.tools[toolID] = "maintenance"
	return nil
}
func (c *Cabinet) FinishMaintenance(toolID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.tools[toolID]; !ok {
		return errors.New("tool not found")
	}
	c.tools[toolID] = "available"
	return nil
}
func (c *Cabinet) IsAvailable(toolID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tools[toolID] == "available"
}
func (c *Cabinet) DueAt(now time.Time, duration time.Duration) time.Time { return now.Add(duration) }
