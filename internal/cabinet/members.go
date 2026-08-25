package cabinet

import (
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrMemberInactive = errors.New("member inactive")

type Members struct {
	mu   sync.Mutex
	data map[string]Member
}

func NewMembers() *Members { return &Members{data: make(map[string]Member)} }
func (m *Members) Register(id, name, phone string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(name) == "" {
		return errors.New("member id and name required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[id] = Member{ID: id, Name: name, Phone: phone, Active: true, JoinedAt: time.Now()}
	return nil
}
func (m *Members) Deactivate(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	member, ok := m.data[id]
	if !ok {
		return false
	}
	member.Active = false
	m.data[id] = member
	return true
}
func (m *Members) Get(id string) (Member, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	member, ok := m.data[id]
	return member, ok
}
func (m *Members) CanBorrow(id string) bool { member, ok := m.Get(id); return ok && member.Active }
