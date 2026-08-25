package cabinet

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

type FeedbackStatus string

const (
	FeedbackVisible  FeedbackStatus = "visible"
	FeedbackHidden   FeedbackStatus = "hidden"
	FeedbackReviewed FeedbackStatus = "reviewed"
)

type Feedback struct {
	ID         string
	ToolID     string
	MemberID   string
	LoanID     string
	Rating     int
	Text       string
	Status     FeedbackStatus
	CreatedAt  time.Time
	ReviewedAt *time.Time
	Reviewer   string
	Reply      string
}
type FeedbackBook struct {
	mu       sync.Mutex
	records  map[string]Feedback
	sequence uint64
}

var ErrFeedbackInput = errors.New("feedback input rejected")

func NewFeedbackBook() *FeedbackBook { return &FeedbackBook{records: make(map[string]Feedback)} }
func (b *FeedbackBook) Create(toolID, memberID, loanID string, rating int, text string, now time.Time) (Feedback, error) {
	text = strings.TrimSpace(text)
	if toolID == "" || memberID == "" || loanID == "" || rating < 1 || rating > 5 || len([]rune(text)) > 500 {
		return Feedback{}, ErrFeedbackInput
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sequence++
	feedback := Feedback{ID: "feedback-" + formatSequence(b.sequence), ToolID: toolID, MemberID: memberID, LoanID: loanID, Rating: rating, Text: text, Status: FeedbackVisible, CreatedAt: now}
	b.records[feedback.ID] = feedback
	return feedback, nil
}
func (b *FeedbackBook) Review(id, reviewer, reply string, visible bool, now time.Time) (Feedback, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	feedback, ok := b.records[id]
	if !ok {
		return Feedback{}, ErrFeedbackInput
	}
	feedback.Reviewer = reviewer
	feedback.Reply = strings.TrimSpace(reply)
	feedback.ReviewedAt = &now
	feedback.Status = FeedbackReviewed
	if !visible {
		feedback.Status = FeedbackHidden
	}
	b.records[id] = feedback
	return feedback, nil
}
func (b *FeedbackBook) ForTool(toolID string, includeHidden bool) []Feedback {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Feedback, 0)
	for _, feedback := range b.records {
		if feedback.ToolID != toolID {
			continue
		}
		// 审核标记为隐藏的反馈不得污染公开查询；仅当显式要求包含隐藏时才保留。
		if !includeHidden && feedback.Status == FeedbackHidden {
			continue
		}
		result = append(result, feedback)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}
func (b *FeedbackBook) Average(toolID string) float64 {
	records := b.ForTool(toolID, false)
	if len(records) == 0 {
		return 0
	}
	total := 0
	for _, feedback := range records {
		total += feedback.Rating
	}
	return float64(total) / float64(len(records))
}
