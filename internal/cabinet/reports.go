package cabinet

import (
	"sort"
	"time"
)

type ToolReport struct {
	ToolID         string
	State          string
	TotalLoans     int
	ActiveLoans    int
	CompletedLoans int
	Utilization    float64
	LastActivity   time.Time
}
type MemberReport struct {
	MemberID             string
	Active               bool
	ActiveLoans          int
	TotalLoans           int
	OutstandingFineCents int64
	LastBorrowedAt       *time.Time
}
type OperationsReport struct {
	GeneratedAt         time.Time
	Tools               []ToolReport
	Members             []MemberReport
	OpenFines           int
	OpenFineCents       int64
	PendingReservations int
	LockerFaults        int
}

func (s *Service) BuildOperationsReport(now time.Time) OperationsReport {
	report := OperationsReport{GeneratedAt: now}
	tools := s.cabinet.Find("")
	for _, tool := range tools {
		report.Tools = append(report.Tools, s.toolReport(tool, now))
	}
	for _, member := range s.members.All() {
		report.Members = append(report.Members, s.memberReport(member))
	}
	for _, fine := range s.fines.All() {
		if fine.Status == FineOpen {
			report.OpenFines++
			report.OpenFineCents += fine.AmountCents
		}
	}
	report.PendingReservations = s.reservations.CountActive("")
	for _, compartment := range s.locker.Snapshot() {
		if compartment.State == CompartmentFault {
			report.LockerFaults++
		}
	}
	sort.Slice(report.Tools, func(i, j int) bool { return report.Tools[i].ToolID < report.Tools[j].ToolID })
	sort.Slice(report.Members, func(i, j int) bool { return report.Members[i].MemberID < report.Members[j].MemberID })
	return report
}
func (s *Service) toolReport(tool Tool, now time.Time) ToolReport {
	loans := s.loans.ForTool(tool.ID)
	result := ToolReport{ToolID: tool.ID, State: tool.State}
	for _, loan := range loans {
		result.TotalLoans++
		result.LastActivity = later(result.LastActivity, loan.ReservedAt)
		if loan.Status == "returned" {
			result.CompletedLoans++
		} else {
			result.ActiveLoans++
		}
	}
	if result.TotalLoans > 0 {
		elapsed := now.Sub(time.Now().Add(-30 * 24 * time.Hour))
		if elapsed > 0 {
			result.Utilization = float64(result.TotalLoans) / 30
		}
	}
	return result
}
func (s *Service) memberReport(member Member) MemberReport {
	loans := s.loans.AllForMember(member.ID)
	result := MemberReport{MemberID: member.ID, Active: member.Active, ActiveLoans: len(s.loans.Active(member.ID)), TotalLoans: len(loans), OutstandingFineCents: s.fines.OutstandingCents(member.ID)}
	for _, loan := range loans {
		if result.LastBorrowedAt == nil || loan.ReservedAt.After(*result.LastBorrowedAt) {
			value := loan.ReservedAt
			result.LastBorrowedAt = &value
		}
	}
	return result
}
func later(a, b time.Time) time.Time {
	if a.IsZero() || b.After(a) {
		return b
	}
	return a
}
