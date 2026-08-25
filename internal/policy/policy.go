package policy

import (
	"errors"
	"strings"
	"time"
)

var ErrMemberLimit = errors.New("member borrowing limit reached")
var ErrPhoneRequired = errors.New("member phone is required")
var ErrInvalidDuration = errors.New("borrowing duration is outside policy")

type Member struct {
	ID     string
	Active bool
	Phone  string
}
type Loan struct {
	ToolID   string
	MemberID string
	DueAt    time.Time
}
type Decision struct {
	Allowed bool
	Reason  string
	DueAt   time.Time
}
type Rules struct {
	MaxActiveLoans int
	DefaultDays    int
	MaxDays        int
	RequirePhone   bool
}

func DefaultRules() Rules {
	return Rules{MaxActiveLoans: 2, DefaultDays: 3, MaxDays: 14, RequirePhone: true}
}
func (r Rules) Evaluate(member Member, active []Loan, requestedDays int, now time.Time) Decision {
	if !member.Active {
		return Decision{Reason: "member inactive"}
	}
	if r.RequirePhone && strings.TrimSpace(member.Phone) == "" {
		return Decision{Reason: ErrPhoneRequired.Error()}
	}
	if len(active) >= r.MaxActiveLoans {
		return Decision{Reason: ErrMemberLimit.Error()}
	}
	days := requestedDays
	if days <= 0 {
		days = r.DefaultDays
	}
	if days > r.MaxDays {
		return Decision{Reason: ErrInvalidDuration.Error()}
	}
	return Decision{Allowed: true, DueAt: now.Add(time.Duration(days) * 24 * time.Hour)}
}
