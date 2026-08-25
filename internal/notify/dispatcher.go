package notify

import (
	"fmt"
	"sync"
	"time"
)

type Message struct {
	At       time.Time
	MemberID string
	Kind     string
	Text     string
}
type Sender interface{ Send(Message) error }
type MemorySender struct {
	mu       sync.Mutex
	messages []Message
}

func NewMemorySender() *MemorySender { return &MemorySender{messages: make([]Message, 0, 32)} }
func (s *MemorySender) Send(message Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, message)
	return nil
}
func (s *MemorySender) Messages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Message, len(s.messages))
	copy(result, s.messages)
	return result
}

type Dispatcher struct{ sender Sender }

func NewDispatcher(sender Sender) *Dispatcher { return &Dispatcher{sender: sender} }
func (d *Dispatcher) LoanOpened(memberID, toolName string, dueAt time.Time) error {
	return d.sender.Send(Message{At: time.Now(), MemberID: memberID, Kind: "loan.opened", Text: fmt.Sprintf("已借出%s，归还时间为%s", toolName, dueAt.Format("2006-01-02 15:04"))})
}
func (d *Dispatcher) LoanReturned(memberID, toolName string) error {
	return d.sender.Send(Message{At: time.Now(), MemberID: memberID, Kind: "loan.returned", Text: fmt.Sprintf("%s已归还", toolName)})
}
func (d *Dispatcher) Overdue(memberID, loanID string) error {
	return d.sender.Send(Message{At: time.Now(), MemberID: memberID, Kind: "loan.overdue", Text: fmt.Sprintf("借用记录%s已逾期，请尽快归还", loanID)})
}
