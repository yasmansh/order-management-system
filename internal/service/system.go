package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"order-management/internal/model"
)

type OrderSystem struct {
	mu        sync.RWMutex
	branches  map[string]*Branch
	employees map[string]*Employee
	orders    map[string]string // order ID -> owning branch ID

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	shutdownOnce sync.Once
	closed       bool
}

func NewOrderSystem(parent context.Context) (*OrderSystem, error) {
	if parent == nil {
		return nil, fmt.Errorf("%w: nil parent context", ErrInvalidArgument)
	}
	ctx, cancel := context.WithCancel(parent)
	return &OrderSystem{
		branches: make(map[string]*Branch), employees: make(map[string]*Employee),
		orders: make(map[string]string), ctx: ctx, cancel: cancel,
	}, nil
}

func normalizeName(value string) string {
	return strings.TrimSpace(value)
}

func (s *OrderSystem) ensureOpenLocked() error {
	if s.closed {
		return ErrSystemClosed
	}
	return nil
}

func (s *OrderSystem) CreateBranch(id, name string) (*Branch, error) {
	if s == nil {
		return nil, ErrInvalidArgument
	}
	id, name = normalizeName(id), normalizeName(name)
	if id == "" || name == "" {
		return nil, ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpenLocked(); err != nil {
		return nil, err
	}
	if _, exists := s.branches[id]; exists {
		return nil, ErrAlreadyExists
	}
	branch := newBranch(id, name)
	s.branches[id] = branch
	// Add to the WaitGroup while holding the system lock. Shutdown takes the
	// same lock before marking the system closed, so Add cannot race with Wait.
	s.wg.Add(1)
	go branch.RunEventDispatcher(s.ctx, &s.wg)
	return branch, nil
}

func (s *OrderSystem) GetBranch(id string) (*Branch, error) {
	if s == nil {
		return nil, ErrInvalidArgument
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	branch, ok := s.branches[id]
	if !ok {
		return nil, ErrNotFound
	}
	return branch, nil
}

func (s *OrderSystem) RegisterEmployee(id, name string, role model.Role, branchID string) (*Employee, error) {
	if s == nil {
		return nil, ErrInvalidArgument
	}
	id, name, branchID = normalizeName(id), normalizeName(name), normalizeName(branchID)
	if id == "" || name == "" || !role.IsValid() || branchID == "" {
		return nil, ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpenLocked(); err != nil {
		return nil, err
	}
	if _, exists := s.employees[id]; exists {
		return nil, ErrAlreadyExists
	}
	branch, ok := s.branches[branchID]
	if !ok {
		return nil, ErrNotFound
	}
	employee := &Employee{id: id, name: name, role: role, currentBranch: branch, events: make(chan model.OrderEvent, defaultEventBuffer)}
	branch.mu.Lock()
	branch.employees[id] = employee
	branch.mu.Unlock()
	s.employees[id] = employee
	return employee, nil
}

func (s *OrderSystem) GetEmployee(id string) (*Employee, error) {
	if s == nil {
		return nil, ErrInvalidArgument
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	employee, ok := s.employees[id]
	if !ok {
		return nil, ErrNotFound
	}
	return employee, nil
}

// SwitchEmployeeBranch serializes a move against broadcasts by locking the
// employee and both branch membership maps. Branch locks are always acquired
// in sorted ID order to prevent deadlocks when two transfers cross paths.
// Events already queued before the move are intentionally not drained.
func (s *OrderSystem) SwitchEmployeeBranch(employeeID, newBranchID string) error {
	if s == nil {
		return ErrInvalidArgument
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrSystemClosed
	}
	employee, employeeOK := s.employees[employeeID]
	newBranch, branchOK := s.branches[newBranchID]
	if !employeeOK || !branchOK {
		return ErrNotFound
	}

	// Keep the system read lock until membership is updated. Shutdown needs
	// the write lock to close the system, so it cannot finish while a transfer
	// that already passed the open check is still in progress.
	employee.mu.Lock()
	defer employee.mu.Unlock()
	oldBranch := employee.currentBranch
	if oldBranch == nil {
		return ErrEmployeeNotAssigned
	}
	if oldBranch == newBranch {
		return ErrSameBranch
	}

	first, second := oldBranch, newBranch
	if first.id > second.id {
		first, second = second, first
	}
	first.mu.Lock()
	second.mu.Lock()
	delete(oldBranch.employees, employeeID)
	newBranch.employees[employeeID] = employee
	employee.currentBranch = newBranch
	second.mu.Unlock()
	first.mu.Unlock()
	return nil
}

func (s *OrderSystem) CreateOrder(branchID, orderID, customer string, items []model.OrderItem) (model.Order, error) {
	if s == nil {
		return model.Order{}, ErrInvalidArgument
	}
	branchID, orderID, customer = normalizeName(branchID), normalizeName(orderID), normalizeName(customer)
	if branchID == "" || orderID == "" || customer == "" || len(items) == 0 {
		return model.Order{}, ErrInvalidArgument
	}
	for _, item := range items {
		if normalizeName(item.Name) == "" || item.Quantity <= 0 {
			return model.Order{}, ErrInvalidArgument
		}
	}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return model.Order{}, ErrSystemClosed
	}
	branch, ok := s.branches[branchID]
	s.mu.RUnlock()
	if !ok {
		return model.Order{}, ErrNotFound
	}

	now := time.Now().UTC()
	order := model.Order{ID: orderID, BranchID: branchID, Customer: customer, Items: append([]model.OrderItem(nil), items...), Status: model.OrderPending, CreatedAt: now, UpdatedAt: now}
	branch.mu.Lock()
	if _, exists := branch.orders[orderID]; exists {
		branch.mu.Unlock()
		return model.Order{}, ErrAlreadyExists
	}
	branch.orders[orderID] = order
	branch.mu.Unlock()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		branch.mu.Lock()
		delete(branch.orders, orderID)
		branch.mu.Unlock()
		return model.Order{}, ErrSystemClosed
	}
	if _, exists := s.orders[orderID]; exists {
		s.mu.Unlock()
		branch.mu.Lock()
		delete(branch.orders, orderID)
		branch.mu.Unlock()
		return model.Order{}, ErrAlreadyExists
	}
	s.orders[orderID] = branchID
	s.mu.Unlock()

	event := newOrderEvent(model.EventOrderCreated, order, "Order created")
	if err := branch.PublishEvent(s.ctx, event); err != nil {
		return order.Clone(), err
	}
	return order.Clone(), nil
}

func (s *OrderSystem) GetOrder(orderID string) (model.Order, error) {
	if s == nil {
		return model.Order{}, ErrInvalidArgument
	}
	s.mu.RLock()
	branchID, ok := s.orders[orderID]
	branch := s.branches[branchID]
	s.mu.RUnlock()
	if !ok || branch == nil {
		return model.Order{}, ErrNotFound
	}
	branch.mu.RLock()
	defer branch.mu.RUnlock()
	order, ok := branch.orders[orderID]
	if !ok {
		return model.Order{}, ErrNotFound
	}
	return order.Clone(), nil
}

func (s *OrderSystem) UpdateOrderStatus(orderID string, next model.OrderStatus) (model.Order, error) {
	if s == nil {
		return model.Order{}, ErrInvalidArgument
	}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return model.Order{}, ErrSystemClosed
	}
	branchID, ok := s.orders[orderID]
	branch := s.branches[branchID]
	s.mu.RUnlock()
	if !ok || branch == nil {
		return model.Order{}, ErrNotFound
	}

	branch.mu.Lock()
	order, ok := branch.orders[orderID]
	if !ok {
		branch.mu.Unlock()
		return model.Order{}, ErrNotFound
	}
	// Validate before mutation. The status machine is defined on the model so
	// all callers follow exactly the same transition rules.
	if !order.Status.CanTransitionTo(next) {
		branch.mu.Unlock()
		return model.Order{}, ErrInvalidTransition
	}
	previous := order.Status
	order.Status = next
	order.UpdatedAt = time.Now().UTC()
	branch.orders[orderID] = order
	branch.mu.Unlock()

	eventType := model.EventOrderStatusChanged
	if next == model.OrderCancelled {
		eventType = model.EventOrderCancelled
	}
	event := newOrderEvent(eventType, order, fmt.Sprintf("Order status changed: %s -> %s", previous, next))
	if err := branch.PublishEvent(s.ctx, event); err != nil {
		return order.Clone(), err
	}
	return order.Clone(), nil
}

func newOrderEvent(eventType model.EventType, order model.Order, message string) model.OrderEvent {
	return model.OrderEvent{ID: fmt.Sprintf("%s-%d", order.ID, time.Now().UnixNano()), Type: eventType, OrderID: order.ID, BranchID: order.BranchID, Message: message, CreatedAt: time.Now().UTC()}
}

// Shutdown first prevents new registrations/branches and then cancels all
// dispatchers. Channel ownership stays with Branch; event channels are not
// closed, avoiding send-on-closed-channel panics during concurrent publishes.
func (s *OrderSystem) Shutdown() {
	if s == nil {
		return
	}
	s.shutdownOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.cancel()
		s.wg.Wait()
	})
}

// BranchIDs is a stable, sorted snapshot useful for diagnostics and examples.
func (s *OrderSystem) BranchIDs() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.branches))
	for id := range s.branches {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
