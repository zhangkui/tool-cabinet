package metrics

import (
	"sync"
	"sync/atomic"
)

type Snapshot struct {
	Reservations   uint64
	Cancellations  uint64
	Checkouts      uint64
	Returns        uint64
	Rejections     uint64
	OverdueNotices uint64
}
type Metrics struct {
	reservations   atomic.Uint64
	cancellations  atomic.Uint64
	checkouts      atomic.Uint64
	returns        atomic.Uint64
	rejections     atomic.Uint64
	overdueNotices atomic.Uint64
	mu             sync.Mutex
}

func New() *Metrics               { return &Metrics{} }
func (m *Metrics) Reservation()   { m.reservations.Add(1) }
func (m *Metrics) Cancellation()  { m.cancellations.Add(1) }
func (m *Metrics) Checkout()      { m.checkouts.Add(1) }
func (m *Metrics) Return()        { m.returns.Add(1) }
func (m *Metrics) Rejection()     { m.rejections.Add(1) }
func (m *Metrics) OverdueNotice() { m.overdueNotices.Add(1) }
func (m *Metrics) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Snapshot{Reservations: m.reservations.Load(), Cancellations: m.cancellations.Load(), Checkouts: m.checkouts.Load(), Returns: m.returns.Load(), Rejections: m.rejections.Load(), OverdueNotices: m.overdueNotices.Load()}
}
