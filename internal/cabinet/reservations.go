package cabinet

import (
	"errors"
	"sort"
	"sync"
	"time"
)

type ReservationStatus string

const (
	ReservationPending   ReservationStatus = "pending"
	ReservationConfirmed ReservationStatus = "confirmed"
	ReservationCancelled ReservationStatus = "cancelled"
	ReservationExpired   ReservationStatus = "expired"
	ReservationFulfilled ReservationStatus = "fulfilled"
)

var ErrReservationNotFound = errors.New("reservation not found")
var ErrReservationTransition = errors.New("reservation transition rejected")
var ErrReservationConflict = errors.New("reservation conflicts with another booking")

type Reservation struct {
	ID          string
	ToolID      string
	MemberID    string
	Status      ReservationStatus
	CreatedAt   time.Time
	StartAt     time.Time
	ExpiresAt   time.Time
	ConfirmedAt *time.Time
	CancelledAt *time.Time
	Note        string
}
type ReservationBook struct {
	mu       sync.RWMutex
	records  map[string]Reservation
	sequence uint64
}

func NewReservationBook() *ReservationBook {
	return &ReservationBook{records: make(map[string]Reservation)}
}
func (b *ReservationBook) Create(toolID, memberID, note string, startAt, expiresAt time.Time) (Reservation, error) {
	if toolID == "" || memberID == "" || !expiresAt.After(startAt) {
		return Reservation{}, ErrReservationTransition
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, existing := range b.records {
		if existing.ToolID == toolID && activeReservation(existing.Status) && overlaps(existing.StartAt, existing.ExpiresAt, startAt, expiresAt) {
			return Reservation{}, ErrReservationConflict
		}
	}
	b.sequence++
	record := Reservation{ID: reservationID(b.sequence), ToolID: toolID, MemberID: memberID, Status: ReservationPending, CreatedAt: time.Now(), StartAt: startAt, ExpiresAt: expiresAt, Note: note}
	b.records[record.ID] = record
	return record, nil
}
func (b *ReservationBook) Confirm(id string, now time.Time) (Reservation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	record, ok := b.records[id]
	if !ok {
		return Reservation{}, ErrReservationNotFound
	}
	if record.Status != ReservationPending || now.After(record.ExpiresAt) {
		return Reservation{}, ErrReservationTransition
	}
	record.Status = ReservationConfirmed
	record.ConfirmedAt = timePtr(now)
	b.records[id] = record
	return record, nil
}
func (b *ReservationBook) Cancel(id string, now time.Time) (Reservation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	record, ok := b.records[id]
	if !ok {
		return Reservation{}, ErrReservationNotFound
	}
	if record.Status != ReservationPending && record.Status != ReservationConfirmed {
		return Reservation{}, ErrReservationTransition
	}
	record.Status = ReservationCancelled
	record.CancelledAt = timePtr(now)
	b.records[id] = record
	return record, nil
}
func (b *ReservationBook) Fulfill(id string, now time.Time) (Reservation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	record, ok := b.records[id]
	if !ok {
		return Reservation{}, ErrReservationNotFound
	}
	if record.Status != ReservationConfirmed {
		return Reservation{}, ErrReservationTransition
	}
	if now.Before(record.StartAt) {
		return Reservation{}, ErrReservationTransition
	}
	record.Status = ReservationFulfilled
	b.records[id] = record
	return record, nil
}
func (b *ReservationBook) Expire(now time.Time) []Reservation {
	b.mu.Lock()
	defer b.mu.Unlock()
	expired := make([]Reservation, 0)
	for id, record := range b.records {
		if (record.Status == ReservationPending || record.Status == ReservationConfirmed) && now.After(record.ExpiresAt) {
			record.Status = ReservationExpired
			b.records[id] = record
			expired = append(expired, record)
		}
	}
	sort.Slice(expired, func(i, j int) bool { return expired[i].ExpiresAt.Before(expired[j].ExpiresAt) })
	return expired
}
func (b *ReservationBook) Get(id string) (Reservation, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	record, ok := b.records[id]
	return record, ok
}
func (b *ReservationBook) List(memberID string, statuses ...ReservationStatus) []Reservation {
	b.mu.RLock()
	defer b.mu.RUnlock()
	allowed := make(map[ReservationStatus]bool)
	for _, status := range statuses {
		allowed[status] = true
	}
	result := make([]Reservation, 0)
	for _, record := range b.records {
		if memberID != "" && record.MemberID != memberID {
			continue
		}
		if len(allowed) > 0 && !allowed[record.Status] {
			continue
		}
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StartAt.Before(result[j].StartAt) })
	return result
}
func (b *ReservationBook) NextForTool(toolID string, now time.Time) (Reservation, bool) {
	records := b.List("", ReservationPending, ReservationConfirmed)
	for _, record := range records {
		if record.ToolID == toolID && !record.StartAt.Before(now) {
			return record, true
		}
	}
	return Reservation{}, false
}
func (b *ReservationBook) CountActive(memberID string) int {
	return len(b.List(memberID, ReservationPending, ReservationConfirmed))
}
func activeReservation(status ReservationStatus) bool {
	return status == ReservationPending || status == ReservationConfirmed
}
func overlaps(startA, endA, startB, endB time.Time) bool {
	return startA.Before(endB) || startB.Before(endA)
}
func reservationID(sequence uint64) string { return "reservation-" + formatSequence(sequence) }
