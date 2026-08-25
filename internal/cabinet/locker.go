package cabinet

import (
	"errors"
	"sort"
	"sync"
	"time"
)

type CompartmentState string

const (
	CompartmentClosed   CompartmentState = "closed"
	CompartmentOpen     CompartmentState = "open"
	CompartmentFault    CompartmentState = "fault"
	CompartmentDisabled CompartmentState = "disabled"
)

type Compartment struct {
	ID           string
	ToolID       string
	State        CompartmentState
	LastOpenedAt *time.Time
	LastClosedAt *time.Time
	OpenCount    uint64
	FaultReason  string
}
type LockerEvent struct {
	At            time.Time
	CompartmentID string
	ToolID        string
	Action        string
	Actor         string
	Detail        string
}
type Locker struct {
	mu           sync.Mutex
	compartments map[string]Compartment
	events       []LockerEvent
}

var ErrCompartmentNotFound = errors.New("compartment not found")
var ErrCompartmentState = errors.New("compartment state rejected")

func NewLocker() *Locker {
	return &Locker{compartments: map[string]Compartment{"slot-a01": {ID: "slot-a01", ToolID: "drill-01", State: CompartmentClosed}, "slot-a02": {ID: "slot-a02", ToolID: "ladder-01", State: CompartmentClosed}}, events: make([]LockerEvent, 0, 64)}
}
func (l *Locker) Assign(compartmentID, toolID, actor string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.compartments[compartmentID]; ok {
		return errors.New("compartment already assigned")
	}
	l.compartments[compartmentID] = Compartment{ID: compartmentID, ToolID: toolID, State: CompartmentClosed}
	l.record(compartmentID, toolID, "assign", actor, "compartment assigned")
	return nil
}
func (l *Locker) OpenForBorrow(toolID, actor string, now time.Time) (Compartment, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	compartment, ok := l.findTool(toolID)
	if !ok {
		return Compartment{}, ErrCompartmentNotFound
	}
	if compartment.State != CompartmentClosed {
		return Compartment{}, ErrCompartmentState
	}
	compartment.State = CompartmentOpen
	compartment.LastOpenedAt = &now
	compartment.OpenCount++
	l.compartments[compartment.ID] = compartment
	l.record(compartment.ID, toolID, "open.borrow", actor, "door opened for pickup")
	return compartment, nil
}
func (l *Locker) Close(compartmentID, actor string, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	compartment, ok := l.compartments[compartmentID]
	if !ok {
		return ErrCompartmentNotFound
	}
	if compartment.State != CompartmentOpen {
		return ErrCompartmentState
	}
	compartment.State = CompartmentClosed
	compartment.LastClosedAt = &now
	l.compartments[compartmentID] = compartment
	l.record(compartmentID, compartment.ToolID, "close", actor, "door closed")
	return nil
}
func (l *Locker) ReportFault(compartmentID, actor, reason string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	compartment, ok := l.compartments[compartmentID]
	if !ok {
		return ErrCompartmentNotFound
	}
	compartment.State = CompartmentClosed
	compartment.FaultReason = reason
	l.compartments[compartmentID] = compartment
	l.record(compartmentID, compartment.ToolID, "fault", actor, reason)
	return nil
}
func (l *Locker) Repair(compartmentID, actor string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	compartment, ok := l.compartments[compartmentID]
	if !ok {
		return ErrCompartmentNotFound
	}
	if compartment.State != CompartmentFault {
		return ErrCompartmentState
	}
	compartment.State = CompartmentClosed
	compartment.FaultReason = ""
	l.compartments[compartmentID] = compartment
	l.record(compartmentID, compartment.ToolID, "repair", actor, "compartment restored")
	return nil
}
func (l *Locker) Disable(compartmentID, actor string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	compartment, ok := l.compartments[compartmentID]
	if !ok {
		return ErrCompartmentNotFound
	}
	compartment.State = CompartmentDisabled
	l.compartments[compartmentID] = compartment
	l.record(compartmentID, compartment.ToolID, "disable", actor, "compartment disabled")
	return nil
}
func (l *Locker) Snapshot() []Compartment {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]Compartment, 0, len(l.compartments))
	for _, compartment := range l.compartments {
		result = append(result, compartment)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
func (l *Locker) Events(limit int) []LockerEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit <= 0 || limit > len(l.events) {
		limit = len(l.events)
	}
	result := make([]LockerEvent, 0, limit)
	for i := len(l.events) - 1; i >= 0 && len(result) < limit; i-- {
		result = append(result, l.events[i])
	}
	return result
}
func (l *Locker) findTool(toolID string) (Compartment, bool) {
	for _, compartment := range l.compartments {
		if compartment.ToolID == toolID {
			return compartment, true
		}
	}
	return Compartment{}, false
}
func (l *Locker) record(compartmentID, toolID, action, actor, detail string) {
	l.events = append(l.events, LockerEvent{At: time.Now(), CompartmentID: compartmentID, ToolID: toolID, Action: action, Actor: actor, Detail: detail})
}
