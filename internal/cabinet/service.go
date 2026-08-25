package cabinet

import (
	"context"
	"errors"
	"example.com/tool-cabinet/internal/metrics"
	"example.com/tool-cabinet/internal/notify"
	"example.com/tool-cabinet/internal/policy"
	"example.com/tool-cabinet/internal/storage"
	"os"
	"time"
)

var ErrInvalidMember = errors.New("member is not allowed to borrow")
var ErrInvalidLoan = errors.New("loan operation rejected")

type Service struct {
	cabinet      *Cabinet
	members      *Members
	loans        *LoanBook
	audit        *AuditLog
	rules        policy.Rules
	dispatcher   *notify.Dispatcher
	sender       *notify.MemorySender
	metrics      *metrics.Metrics
	store        *storage.FileStore
	reservations *ReservationBook
	locker       *Locker
	fines        *FineLedger
	inspections  *InspectionBook
	feedback     *FeedbackBook
	waitlist     *Waitlist
	scheduler    *Scheduler
}

func NewService() *Service {
	sender := notify.NewMemorySender()
	service := &Service{cabinet: New(), members: NewMembers(), loans: NewLoanBook(), audit: NewAuditLog(), rules: policy.DefaultRules(), dispatcher: notify.NewDispatcher(sender), sender: sender, metrics: metrics.New(), reservations: NewReservationBook(), locker: NewLocker(), fines: NewFineLedger(), inspections: NewInspectionBook(), feedback: NewFeedbackBook(), waitlist: NewWaitlist(), scheduler: NewScheduler()}
	if path := os.Getenv("TOOL_CABINET_STATE"); path != "" {
		service.store = storage.NewFileStore(path)
	}
	return service
}
func (s *Service) RegisterMember(id, name, phone string) error {
	if err := s.members.Register(id, name, phone); err != nil {
		return err
	}
	s.audit.Append(id, "member.register", "", name)
	return nil
}

func (s *Service) Reserve(ctx context.Context, memberID, toolID string, hold time.Duration) error {
	if !s.allowed(memberID, 0) {
		s.metrics.Rejection()
		return ErrInvalidMember
	}
	if !s.cabinet.IsAvailable(toolID) {
		s.metrics.Rejection()
		return ErrUnavailable
	}
	s.metrics.Reservation()
	if err := s.cabinet.Reserve(ctx, toolID, hold); err != nil {
		if errors.Is(err, context.Canceled) {
			s.metrics.Cancellation()
		}
		return err
	}
	loan, err := s.loans.Open(toolID, memberID, time.Duration(s.rules.DefaultDays)*24*time.Hour)
	if err != nil {
		return err
	}
	s.audit.Append(memberID, "loan.open", toolID, loan.ID)
	_ = s.dispatcher.LoanOpened(memberID, displayName(toolID), loan.DueAt)
	s.metrics.Checkout()
	s.persist()
	return nil
}

func (s *Service) Checkout(memberID, toolID string, days int) (Loan, error) {
	if !s.allowed(memberID, days) {
		s.metrics.Rejection()
		return Loan{}, ErrInvalidMember
	}
	if !s.cabinet.IsAvailable(toolID) {
		s.metrics.Rejection()
		return Loan{}, ErrUnavailable
	}
	decision := s.decision(memberID, days)
	if !decision.Allowed {
		s.metrics.Rejection()
		return Loan{}, errors.New(decision.Reason)
	}
	if err := s.cabinet.Reserve(context.Background(), toolID, 0); err != nil {
		s.metrics.Rejection()
		return Loan{}, err
	}
	loan, err := s.loans.Open(toolID, memberID, time.Until(decision.DueAt))
	if err != nil {
		_ = s.cabinet.Release(toolID)
		return Loan{}, err
	}
	s.audit.Append(memberID, "loan.checkout", toolID, loan.ID)
	_ = s.dispatcher.LoanOpened(memberID, displayName(toolID), loan.DueAt)
	s.metrics.Checkout()
	s.persist()
	return loan, nil
}

func (s *Service) Return(loanID, memberID string) (Loan, error) {
	loan, err := s.loans.Close(loanID)
	if err != nil {
		return Loan{}, err
	}
	if memberID != "" && loan.MemberID != memberID {
		return Loan{}, ErrInvalidLoan
	}
	if err := s.cabinet.Release(loan.ToolID); err != nil {
		return Loan{}, err
	}
	s.audit.Append(memberID, "loan.return", loan.ToolID, loan.ID)
	_ = s.dispatcher.LoanReturned(memberID, displayName(loan.ToolID))
	s.metrics.Return()
	s.persist()
	return loan, nil
}

func (s *Service) SetMaintenance(toolID string, enabled bool, actor string) error {
	var err error
	if enabled {
		err = s.cabinet.StartMaintenance(toolID)
	} else {
		err = s.cabinet.FinishMaintenance(toolID)
	}
	if err == nil {
		action := "maintenance.start"
		if !enabled {
			action = "maintenance.finish"
		}
		s.audit.Append(actor, action, toolID, "状态已更新")
		s.persist()
	}
	return err
}
func (s *Service) Status(toolID string) (string, bool) { return s.cabinet.Status(toolID) }
func (s *Service) Catalog(query string) []Tool         { return s.cabinet.Find(query) }
func (s *Service) ActiveLoans(memberID string) []Loan  { return s.loans.Active(memberID) }
func (s *Service) OverdueLoans() []Loan {
	result := s.loans.Overdue(time.Now())
	for _, loan := range result {
		_ = s.dispatcher.Overdue(loan.MemberID, loan.ID)
		s.metrics.OverdueNotice()
	}
	return result
}
func (s *Service) Audit(limit int) []AuditEvent           { return s.audit.Recent(limit) }
func (s *Service) Notifications() []notify.Message        { return s.sender.Messages() }
func (s *Service) Metrics() metrics.Snapshot              { return s.metrics.Snapshot() }
func (s *Service) allowed(memberID string, days int) bool { return s.decision(memberID, days).Allowed }
func (s *Service) decision(memberID string, days int) policy.Decision {
	member, ok := s.members.Get(memberID)
	if !ok {
		return policy.Decision{Reason: ErrInvalidMember.Error()}
	}
	active := s.loans.Active(memberID)
	loans := make([]policy.Loan, 0, len(active))
	for _, loan := range active {
		loans = append(loans, policy.Loan{ToolID: loan.ToolID, MemberID: loan.MemberID, DueAt: loan.DueAt})
	}
	return s.rules.Evaluate(policy.Member{ID: member.ID, Active: member.Active, Phone: member.Phone}, loans, days, time.Now())
}
func (s *Service) persist() {
	if s.store == nil {
		return
	}
	tools := s.Catalog("")
	records := make([]storage.ToolRecord, 0, len(tools))
	for _, tool := range tools {
		records = append(records, storage.ToolRecord{ID: tool.ID, State: tool.State, Condition: tool.Condition, UpdatedAt: tool.UpdatedAt})
	}
	_ = s.store.Save(storage.State{Tools: records})
}

func (s *Service) JoinWaitlist(toolID, memberID string, priority WaitlistPriority, duration time.Duration, note string) (WaitlistEntry, error) {
	if !s.members.CanBorrow(memberID) {
		return WaitlistEntry{}, ErrInvalidMember
	}
	entry, err := s.waitlist.Join(toolID, memberID, priority, duration, note)
	if err == nil {
		s.audit.Append(memberID, "waitlist.join", toolID, entry.ID)
	}
	return entry, err
}
func (s *Service) LeaveWaitlist(id, actor string) (WaitlistEntry, error) {
	entry, err := s.waitlist.Leave(id)
	if err == nil {
		s.audit.Append(actor, "waitlist.leave", entry.ToolID, entry.ID)
	}
	return entry, err
}
func (s *Service) WaitlistForMember(memberID string) []WaitlistEntry {
	return s.waitlist.ForMember(memberID, false)
}
func (s *Service) EnqueueJob(name string, at time.Time) (Job, error) {
	return s.scheduler.Enqueue(name, at)
}
func (s *Service) RunScheduledJob(ctx context.Context, now time.Time) (Job, error) {
	return s.scheduler.RunOnce(ctx, now, func(ctx context.Context, job Job) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		switch job.Name {
		case "expire-reservations":
			s.reservations.Expire(time.Now())
		case "reconcile-overdue":
			s.OverdueLoans()
		default:
			return errors.New("unsupported scheduled job")
		}
		return nil
	})
}
func (s *Service) Jobs(status JobStatus) []Job { return s.scheduler.List(status) }

func (s *Service) StartInspection(loanID, inspector string, now time.Time) (Inspection, error) {
	loan, ok := s.loans.Get(loanID)
	if !ok {
		return Inspection{}, errors.New("loan not found")
	}
	inspection, err := s.inspections.Start(loan, inspector, now)
	if err == nil {
		s.audit.Append(inspector, "inspection.start", loan.ToolID, inspection.ID)
	}
	return inspection, err
}
func (s *Service) CompleteInspection(id string, grade ConditionGrade, items []InspectionItem, summary string, inspector string, now time.Time) (Inspection, error) {
	inspection, err := s.inspections.Complete(id, grade, items, summary, now)
	if err == nil {
		s.audit.Append(inspector, "inspection.complete", inspection.ToolID, inspection.ID)
		if inspection.Status == InspectionClaim {
			s.audit.Append(inspector, "tool.claim", inspection.ToolID, summary)
		}
	}
	return inspection, err
}
func (s *Service) InspectionsForTool(toolID string) []Inspection {
	return s.inspections.ForTool(toolID)
}
func (s *Service) SubmitFeedback(toolID, memberID, loanID string, rating int, text string, now time.Time) (Feedback, error) {
	feedback, err := s.feedback.Create(toolID, memberID, loanID, rating, text, now)
	if err == nil {
		s.audit.Append(memberID, "feedback.create", toolID, feedback.ID)
	}
	return feedback, err
}
func (s *Service) ToolRating(toolID string) float64        { return s.feedback.Average(toolID) }
func (s *Service) PublicFeedback(toolID string) []Feedback { return s.feedback.ForTool(toolID, false) }
