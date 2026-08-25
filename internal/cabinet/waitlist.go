package cabinet

import (
	"errors"
	"sort"
	"sync"
	"time"
)

type WaitlistPriority int

const (
	PriorityNormal         WaitlistPriority = 10
	PriorityResidentSenior WaitlistPriority = 20
	PriorityUrgent         WaitlistPriority = 30
)

type WaitlistEntry struct {
	ID         string
	ToolID     string
	MemberID   string
	Priority   WaitlistPriority
	JoinedAt   time.Time
	ExpiresAt  time.Time
	NotifiedAt *time.Time
	Attempts   int
	Active     bool
	Note       string
}
type Waitlist struct {
	mu       sync.Mutex
	entries  map[string]WaitlistEntry
	sequence uint64
}

var ErrWaitlistDuplicate = errors.New("member already waiting for tool")
var ErrWaitlistMissing = errors.New("waitlist entry not found")

func NewWaitlist() *Waitlist { return &Waitlist{entries: make(map[string]WaitlistEntry)} }
func (w *Waitlist) Join(toolID, memberID string, priority WaitlistPriority, duration time.Duration, note string) (WaitlistEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	for _, entry := range w.entries {
		if entry.Active && entry.ToolID == toolID && entry.MemberID == memberID {
			return WaitlistEntry{}, ErrWaitlistDuplicate
		}
	}
	w.sequence++
	entry := WaitlistEntry{ID: "wait-" + formatSequence(w.sequence), ToolID: toolID, MemberID: memberID, Priority: priority, JoinedAt: now, ExpiresAt: now.Add(duration), Active: true, Note: note}
	w.entries[entry.ID] = entry
	return entry, nil
}
func (w *Waitlist) Leave(id string) (WaitlistEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	entry, ok := w.entries[id]
	if !ok {
		return WaitlistEntry{}, ErrWaitlistMissing
	}
	if !entry.Active {
		return WaitlistEntry{}, ErrWaitlistMissing
	}
	entry.Active = false
	w.entries[id] = entry
	return entry, nil
}
func (w *Waitlist) Next(toolID string, now time.Time) (WaitlistEntry, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	candidates := make([]WaitlistEntry, 0)
	for id, entry := range w.entries {
		if entry.ToolID != toolID || !entry.Active {
			continue
		}
		if now.After(entry.ExpiresAt) {
			entry.Active = false
			w.entries[id] = entry
			continue
		}
		candidates = append(candidates, entry)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		return candidates[i].JoinedAt.Before(candidates[j].JoinedAt)
	})
	if len(candidates) == 0 {
		return WaitlistEntry{}, false
	}
	return candidates[0], true
}
func (w *Waitlist) Notify(id string, now time.Time) (WaitlistEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	entry, ok := w.entries[id]
	if !ok || !entry.Active {
		return WaitlistEntry{}, ErrWaitlistMissing
	}
	entry.NotifiedAt = &now
	entry.Attempts++
	w.entries[id] = entry
	return entry, nil
}
func (w *Waitlist) Expire(now time.Time) []WaitlistEntry {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make([]WaitlistEntry, 0)
	for id, entry := range w.entries {
		if entry.Active && now.After(entry.ExpiresAt) {
			entry.Active = false
			w.entries[id] = entry
			result = append(result, entry)
		}
	}
	return result
}
func (w *Waitlist) ForMember(memberID string, activeOnly bool) []WaitlistEntry {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make([]WaitlistEntry, 0)
	for _, entry := range w.entries {
		if entry.MemberID == memberID && (!activeOnly || entry.Active) {
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].JoinedAt.Before(result[j].JoinedAt) })
	return result
}
func (w *Waitlist) Count(toolID string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	total := 0
	for _, entry := range w.entries {
		if entry.ToolID == toolID && entry.Active {
			total++
		}
	}
	return total
}
