package cabinet

import (
	"errors"
	"sort"
	"sync"
	"time"
)

type FineStatus string

const (
	FineOpen    FineStatus = "open"
	FineSettled FineStatus = "settled"
	FineWaived  FineStatus = "waived"
)

type Fine struct {
	ID          string
	LoanID      string
	MemberID    string
	AmountCents int64
	Reason      string
	Status      FineStatus
	CreatedAt   time.Time
	SettledAt   *time.Time
	Note        string
}
type FineLedger struct {
	mu       sync.RWMutex
	fines    map[string]Fine
	sequence uint64
}

var ErrFineNotFound = errors.New("fine not found")
var ErrFineState = errors.New("fine state rejected")

func NewFineLedger() *FineLedger { return &FineLedger{fines: make(map[string]Fine)} }
func (l *FineLedger) Assess(loan Loan, now time.Time, dailyRate int64) (Fine, error) {
	if loan.ID == "" || !loan.DueAt.Before(now) {
		return Fine{}, ErrFineState
	}
	days := int64(now.Sub(loan.DueAt).Hours() / 24)
	if days < 1 {
		days = 1
	}
	amount := days * dailyRate
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, existing := range l.fines {
		if existing.LoanID == loan.ID && existing.Status == FineOpen {
			return existing, nil
		}
	}
	l.sequence++
	fine := Fine{ID: fineID(l.sequence), LoanID: loan.ID, MemberID: loan.MemberID, AmountCents: amount, Reason: "逾期归还", Status: FineOpen, CreatedAt: now, Note: "系统根据逾期天数自动计算"}
	l.fines[fine.ID] = fine
	return fine, nil
}
func (l *FineLedger) Settle(id string, now time.Time, note string) (Fine, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fine, ok := l.fines[id]
	if !ok {
		return Fine{}, ErrFineNotFound
	}
	if fine.Status != FineOpen {
		return Fine{}, ErrFineState
	}
	fine.Status = FineSettled
	fine.SettledAt = &now
	fine.Note = note
	l.fines[id] = fine
	return fine, nil
}
func (l *FineLedger) Waive(id string, now time.Time, operator, reason string) (Fine, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fine, ok := l.fines[id]
	if !ok {
		return Fine{}, ErrFineNotFound
	}
	if fine.Status != FineOpen {
		return Fine{}, ErrFineState
	}
	fine.Status = FineWaived
	fine.SettledAt = &now
	fine.Note = operator + ":" + reason
	l.fines[id] = fine
	return fine, nil
}
func (l *FineLedger) Get(id string) (Fine, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	fine, ok := l.fines[id]
	return fine, ok
}
func (l *FineLedger) ForMember(memberID string, includeClosed bool) []Fine {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]Fine, 0)
	for _, fine := range l.fines {
		if fine.MemberID != memberID {
			continue
		}
		if !includeClosed && fine.Status != FineOpen {
			continue
		}
		result = append(result, fine)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}
func (l *FineLedger) OutstandingCents(memberID string) int64 {
	var total int64
	for _, fine := range l.ForMember(memberID, false) {
		total += fine.AmountCents
	}
	return total
}
func (l *FineLedger) All() []Fine {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]Fine, 0, len(l.fines))
	for _, fine := range l.fines {
		result = append(result, fine)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}
func fineID(sequence uint64) string { return "fine-" + formatSequence(sequence) }
