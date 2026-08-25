package cabinet

import (
	"sync"
	"time"
)

type AuditLog struct {
	mu     sync.Mutex
	events []AuditEvent
}

func NewAuditLog() *AuditLog { return &AuditLog{events: make([]AuditEvent, 0, 32)} }
func (a *AuditLog) Append(actor, action, toolID, detail string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, AuditEvent{At: time.Now(), Actor: actor, Action: action, ToolID: toolID, Detail: detail})
}
func (a *AuditLog) Recent(limit int) []AuditEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	if limit <= 0 || limit > len(a.events) {
		limit = len(a.events)
	}
	result := make([]AuditEvent, 0, limit)
	for i := len(a.events) - 1; i >= 0 && len(result) < limit; i-- {
		result = append(result, a.events[i])
	}
	return result
}
