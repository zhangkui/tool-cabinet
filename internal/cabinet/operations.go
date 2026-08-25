package cabinet

import "time"

func (m *Members) All() []Member {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Member, 0, len(m.data))
	for _, member := range m.data {
		result = append(result, member)
	}
	return result
}
func (b *LoanBook) Get(id string) (Loan, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	loan, ok := b.loans[id]
	return loan, ok
}
func (b *LoanBook) ForTool(toolID string) []Loan {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Loan, 0)
	for _, loan := range b.loans {
		if loan.ToolID == toolID {
			result = append(result, loan)
		}
	}
	return result
}
func (b *LoanBook) AllForMember(memberID string) []Loan {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Loan, 0)
	for _, loan := range b.loans {
		if loan.MemberID == memberID {
			result = append(result, loan)
		}
	}
	return result
}
func (b *LoanBook) Expired(now time.Time) []Loan {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Loan, 0)
	for _, loan := range b.loans {
		if loan.Status != "returned" && loan.DueAt.Before(now) {
			result = append(result, loan)
		}
	}
	return result
}
