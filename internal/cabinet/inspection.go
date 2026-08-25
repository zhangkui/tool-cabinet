package cabinet

import (
	"errors"
	"sort"
	"sync"
	"time"
)

type ConditionGrade string

const (
	ConditionExcellent ConditionGrade = "excellent"
	ConditionGood      ConditionGrade = "good"
	ConditionWorn      ConditionGrade = "worn"
	ConditionDamaged   ConditionGrade = "damaged"
	ConditionMissing   ConditionGrade = "missing"
)

type InspectionStatus string

const (
	InspectionPending InspectionStatus = "pending"
	InspectionPassed  InspectionStatus = "passed"
	InspectionRepair  InspectionStatus = "repair"
	InspectionClaim   InspectionStatus = "claim"
)

type InspectionItem struct {
	Name      string
	Passed    bool
	Note      string
	PhotoKeys []string
}
type Inspection struct {
	ID                   string
	LoanID               string
	ToolID               string
	MemberID             string
	Status               InspectionStatus
	Grade                ConditionGrade
	Items                []InspectionItem
	EstimatedChargeCents int64
	CreatedAt            time.Time
	CompletedAt          *time.Time
	Inspector            string
	Summary              string
}
type InspectionBook struct {
	mu       sync.Mutex
	records  map[string]Inspection
	sequence uint64
}

var ErrInspectionNotFound = errors.New("inspection not found")
var ErrInspectionCompleted = errors.New("inspection already completed")
var ErrInspectionInput = errors.New("inspection input rejected")

func NewInspectionBook() *InspectionBook {
	return &InspectionBook{records: make(map[string]Inspection)}
}
func (b *InspectionBook) Start(loan Loan, inspector string, now time.Time) (Inspection, error) {
	if loan.ID == "" || inspector == "" {
		return Inspection{}, ErrInspectionInput
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, existing := range b.records {
		if existing.LoanID == loan.ID && existing.Status == InspectionPending {
			return existing, nil
		}
	}
	b.sequence++
	inspection := Inspection{ID: "inspection-" + formatSequence(b.sequence), LoanID: loan.ID, ToolID: loan.ToolID, MemberID: loan.MemberID, Status: InspectionPending, Grade: ConditionGood, CreatedAt: now, Inspector: inspector, Items: defaultInspectionItems()}
	b.records[inspection.ID] = inspection
	return inspection, nil
}
func (b *InspectionBook) Complete(id string, grade ConditionGrade, items []InspectionItem, summary string, now time.Time) (Inspection, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	inspection, ok := b.records[id]
	if !ok {
		return Inspection{}, ErrInspectionNotFound
	}
	if inspection.Status != InspectionPending {
		return Inspection{}, ErrInspectionCompleted
	}
	if !validGrade(grade) || len(items) == 0 {
		return Inspection{}, ErrInspectionInput
	}
	inspection.Grade = grade
	inspection.Items = cloneInspectionItems(items)
	inspection.Summary = summary
	inspection.CompletedAt = &now
	inspection.Status = statusForGrade(grade)
	inspection.EstimatedChargeCents = estimateDamage(grade, items)
	b.records[id] = inspection
	return inspection, nil
}
func (b *InspectionBook) Get(id string) (Inspection, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	inspection, ok := b.records[id]
	return cloneInspection(inspection), ok
}
func (b *InspectionBook) ForTool(toolID string) []Inspection {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Inspection, 0)
	for _, inspection := range b.records {
		if inspection.ToolID == toolID {
			result = append(result, cloneInspection(inspection))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}
func (b *InspectionBook) ForMember(memberID string) []Inspection {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Inspection, 0)
	for _, inspection := range b.records {
		if inspection.MemberID == memberID {
			result = append(result, cloneInspection(inspection))
		}
	}
	return result
}
func (b *InspectionBook) Pending() []Inspection {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Inspection, 0)
	for _, inspection := range b.records {
		if inspection.Status == InspectionPending {
			result = append(result, cloneInspection(inspection))
		}
	}
	return result
}
func defaultInspectionItems() []InspectionItem {
	return []InspectionItem{{Name: "外观完整", Passed: true}, {Name: "电源与电池", Passed: true}, {Name: "安全配件", Passed: true}, {Name: "清洁状态", Passed: true}, {Name: "使用说明", Passed: true}}
}
func validGrade(grade ConditionGrade) bool {
	switch grade {
	case ConditionExcellent, ConditionGood, ConditionWorn, ConditionDamaged, ConditionMissing:
		return true
	default:
		return false
	}
}
func statusForGrade(grade ConditionGrade) InspectionStatus {
	if grade == ConditionMissing {
		return InspectionClaim
	}
	if grade == ConditionWorn {
		return InspectionRepair
	}
	return InspectionPassed
}
func estimateDamage(grade ConditionGrade, items []InspectionItem) int64 {
	base := map[ConditionGrade]int64{ConditionExcellent: 0, ConditionGood: 0, ConditionWorn: 500, ConditionDamaged: 3000, ConditionMissing: 15000}[grade]
	failed := int64(0)
	for _, item := range items {
		if !item.Passed {
			failed++
		}
	}
	return base + failed*300
}
func cloneInspectionItems(items []InspectionItem) []InspectionItem {
	result := make([]InspectionItem, len(items))
	for i, item := range items {
		result[i] = item
		result[i].PhotoKeys = append([]string(nil), item.PhotoKeys...)
	}
	return result
}
func cloneInspection(inspection Inspection) Inspection {
	inspection.Items = cloneInspectionItems(inspection.Items)
	return inspection
}
