package service

import (
	"context"
	"sync"
	"sync/atomic"

	"order-management/internal/model"
)

const defaultEventBuffer = 128

type Branch struct {
	id   string
	name string

	mu        sync.RWMutex
	employees map[string]*Employee
	orders    map[string]model.Order
	events    chan model.OrderEvent

	droppedEvents atomic.Uint64
}

func newBranch(id, name string) *Branch {
	return &Branch{
		id: id, name: name,
		employees: make(map[string]*Employee),
		orders:    make(map[string]model.Order),
		events:    make(chan model.OrderEvent, defaultEventBuffer),
	}
}

func (b *Branch) ID() string {
	if b == nil {
		return ""
	}
	return b.id
}

func (b *Branch) Name() string {
	if b == nil {
		return ""
	}
	return b.name
}

func (b *Branch) DroppedEvents() uint64 {
	if b == nil {
		return 0
	}
	return b.droppedEvents.Load()
}

// PublishEvent only enqueues an event. It deliberately does not iterate over
// employees; routing and delivery belong to the dispatcher/broadcast stages.
func (b *Branch) PublishEvent(ctx context.Context, event model.OrderEvent) error {
	if b == nil || ctx == nil {
		return ErrInvalidArgument
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case b.events <- event:
		return nil
	}
}

// RunEventDispatcher owns the read side of the branch event channel. The
// channel is never closed; cancellation is the single shutdown signal.
func (b *Branch) RunEventDispatcher(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-b.events:
			b.DispatchEvent(ctx, event)
		}
	}
}

// DispatchEvent is the routing layer. This first version broadcasts every
// branch event to every currently assigned employee. Role-based routing can
// be added here later without changing PublishEvent or BroadcastEvent.
func (b *Branch) DispatchEvent(ctx context.Context, event model.OrderEvent) {
	b.BroadcastEvent(ctx, event)
}

// BroadcastEvent sends to the current branch membership only. The read lock
// is intentionally held for the short, non-blocking sends: a transfer must
// wait for an in-flight broadcast, and once it returns later broadcasts see
// the updated membership. A full employee buffer drops the newest event and
// increments a counter instead of stalling the branch dispatcher forever.
func (b *Branch) BroadcastEvent(ctx context.Context, event model.OrderEvent) {
	if b == nil {
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, employee := range b.employees {
		select {
		case <-ctx.Done():
			return
		case employee.events <- event:
		default:
			b.droppedEvents.Add(1)
		}
	}
}
