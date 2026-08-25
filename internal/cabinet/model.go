package cabinet

import "time"

type Tool struct {
	ID        string
	Name      string
	Category  string
	Location  string
	State     string
	Condition string
	UpdatedAt time.Time
}
type Member struct {
	ID       string
	Name     string
	Phone    string
	Active   bool
	JoinedAt time.Time
}
type Loan struct {
	ID         string
	ToolID     string
	MemberID   string
	Status     string
	ReservedAt time.Time
	DueAt      time.Time
	ReturnedAt *time.Time
}
type AuditEvent struct {
	At     time.Time
	Actor  string
	Action string
	ToolID string
	Detail string
}
