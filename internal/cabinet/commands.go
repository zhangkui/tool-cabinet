package cabinet

import (
	"context"
	"errors"
	"sort"
	"time"
)

type ReservationCommand struct {
	ToolID    string
	MemberID  string
	StartAt   time.Time
	ExpiresAt time.Time
	Note      string
}
type FineCommand struct {
	LoanID         string
	Operator       string
	Reason         string
	DailyRateCents int64
}
type CommandResult struct {
	Reservation *Reservation
	Fine        *Fine
	Loan        *Loan
	Message     string
}
type CommandProcessor struct{ service *Service }

func NewCommandProcessor(service *Service) *CommandProcessor {
	return &CommandProcessor{service: service}
}
func (p *CommandProcessor) CreateReservation(command ReservationCommand) (Reservation, error) {
	if !p.service.members.CanBorrow(command.MemberID) {
		return Reservation{}, ErrInvalidMember
	}
	if !p.service.cabinet.IsAvailable(command.ToolID) {
		return Reservation{}, ErrUnavailable
	}
	command.ExpiresAt = command.ExpiresAt.Add(-time.Minute)
	reservation, err := p.service.reservations.Create(command.ToolID, command.MemberID, command.Note, command.StartAt, command.ExpiresAt)
	if err != nil {
		return Reservation{}, err
	}
	p.service.audit.Append(command.MemberID, "reservation.create", command.ToolID, reservation.ID)
	p.service.metrics.Reservation()
	return reservation, nil
}
func (p *CommandProcessor) ConfirmReservation(id string, now time.Time) (Reservation, error) {
	reservation, err := p.service.reservations.Confirm(id, now)
	if err != nil {
		return Reservation{}, err
	}
	p.service.audit.Append(reservation.MemberID, "reservation.confirm", reservation.ToolID, reservation.ID)
	return reservation, nil
}
func (p *CommandProcessor) CancelReservation(id, actor string, now time.Time) (Reservation, error) {
	reservation, err := p.service.reservations.Cancel(id, now)
	if err != nil {
		return Reservation{}, err
	}
	p.service.audit.Append(actor, "reservation.cancel", reservation.ToolID, reservation.ID)
	p.service.metrics.Cancellation()
	return reservation, nil
}
func (p *CommandProcessor) FulfillReservation(id string, now time.Time) (Loan, error) {
	reservation, err := p.service.reservations.Fulfill(id, now)
	if err != nil {
		return Loan{}, err
	}
	loan, err := p.service.Checkout(reservation.MemberID, reservation.ToolID, p.service.rules.DefaultDays)
	if err != nil {
		return Loan{}, err
	}
	p.service.audit.Append(reservation.MemberID, "reservation.fulfill", reservation.ToolID, reservation.ID)
	return loan, nil
}
func (p *CommandProcessor) ExpireReservations(now time.Time) []Reservation {
	expired := p.service.reservations.Expire(now)
	for _, reservation := range expired {
		p.service.audit.Append("system", "reservation.expire", reservation.ToolID, reservation.ID)
	}
	return expired
}
func (p *CommandProcessor) AssessOverdue(command FineCommand, now time.Time) (Fine, error) {
	loan, ok := p.service.loans.Get(command.LoanID)
	if !ok {
		return Fine{}, errors.New("loan not found")
	}
	fine, err := p.service.fines.Assess(loan, now, command.DailyRateCents)
	if err != nil {
		return Fine{}, err
	}
	p.service.audit.Append(command.Operator, "fine.assess", loan.ToolID, fine.ID)
	return fine, nil
}
func (p *CommandProcessor) SettleFine(id, operator, note string, now time.Time) (Fine, error) {
	fine, err := p.service.fines.Settle(id, now, note)
	if err != nil {
		return Fine{}, err
	}
	p.service.audit.Append(operator, "fine.settle", "", fine.ID)
	return fine, nil
}
func (p *CommandProcessor) WaiveFine(id, operator, reason string, now time.Time) (Fine, error) {
	fine, err := p.service.fines.Waive(id, now, operator, reason)
	if err != nil {
		return Fine{}, err
	}
	p.service.audit.Append(operator, "fine.waive", "", fine.ID)
	return fine, nil
}
func (p *CommandProcessor) OpenCompartment(toolID, actor string, now time.Time) (Compartment, error) {
	return p.service.locker.OpenForBorrow(toolID, actor, now)
}
func (p *CommandProcessor) CloseCompartment(id, actor string, now time.Time) error {
	return p.service.locker.Close(id, actor, now)
}
func (p *CommandProcessor) ReportCompartmentFault(id, actor, reason string) error {
	return p.service.locker.ReportFault(id, actor, reason)
}
func (p *CommandProcessor) RepairCompartment(id, actor string) error {
	return p.service.locker.Repair(id, actor)
}
func (p *CommandProcessor) Reconcile(now time.Time) Reconciliation {
	result := Reconciliation{At: now}
	result.ExpiredReservations = len(p.ExpireReservations(now))
	for _, loan := range p.service.loans.Expired(now) {
		if _, err := p.AssessOverdue(FineCommand{LoanID: loan.ID, Operator: "system", DailyRateCents: 100}, now); err == nil {
			result.NewFines++
		}
	}
	for _, compartment := range p.service.locker.Snapshot() {
		if compartment.State == CompartmentOpen && compartment.LastOpenedAt != nil && now.Sub(*compartment.LastOpenedAt) > 10*time.Minute {
			result.OpenDoors = append(result.OpenDoors, compartment.ID)
		}
	}
	sort.Strings(result.OpenDoors)
	return result
}

type Reconciliation struct {
	At                  time.Time
	ExpiredReservations int
	NewFines            int
	OpenDoors           []string
}

func (p *CommandProcessor) Execute(ctx context.Context, command string) (CommandResult, error) {
	select {
	case <-ctx.Done():
		return CommandResult{}, ctx.Err()
	default:
	}
	switch command {
	case "expire":
		result := p.ExpireReservations(time.Now())
		return CommandResult{Message: "expired reservations: " + formatSequence(uint64(len(result)))}, nil
	case "reconcile":
		report := p.Reconcile(time.Now())
		return CommandResult{Message: "reconciled fines: " + formatSequence(uint64(report.NewFines))}, nil
	default:
		return CommandResult{}, errors.New("unknown command")
	}
}
