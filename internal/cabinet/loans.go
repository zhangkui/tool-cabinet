package cabinet

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type LoanBook struct {
	mu    sync.Mutex
	loans map[string]Loan
	next  uint64
}

func NewLoanBook() *LoanBook { return &LoanBook{loans: make(map[string]Loan)} }
func (b *LoanBook) Open(toolID, memberID string, duration time.Duration) (Loan, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, loan := range b.loans {
		if loan.ToolID == toolID && loan.Status != "returned" {
			return Loan{}, errors.New("tool already borrowed")
		}
	}
	b.next++
	now := time.Now()
	loan := Loan{ID: fmt.Sprintf("loan-%04d", b.next), ToolID: toolID, MemberID: memberID, Status: "borrowed", ReservedAt: now, DueAt: now.Add(duration)}
	b.loans[loan.ID] = loan
	return loan, nil
}
func (b *LoanBook) Close(id string) (Loan, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	loan, ok := b.loans[id]
	if !ok {
		return Loan{}, errors.New("loan not found")
	}
	if loan.Status == "returned" {
		return loan, errors.New("loan already returned")
	}
	now := time.Now()
	loan.Status = "returned"
	loan.ReturnedAt = &now
	b.loans[id] = loan
	return loan, nil
}
func (b *LoanBook) Active(memberID string) []Loan {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Loan, 0)
	for _, loan := range b.loans {
		if loan.MemberID == memberID && loan.Status != "returned" {
			result = append(result, loan)
		}
	}
	return result
}
func (b *LoanBook) Overdue(now time.Time) []Loan {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Loan, 0)
	for _, loan := range b.loans {
		if loan.Status == "borrowed" && loan.DueAt.Before(now) {
			result = append(result, loan)
		}
	}
	return result
}
