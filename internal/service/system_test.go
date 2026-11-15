package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"order-management/internal/model"
)

func newTestSystem(t *testing.T) *OrderSystem {
	t.Helper()
	s, err := NewOrderSystem(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Shutdown)
	return s
}

func TestCreateOrderAndStatusChange(t *testing.T) {
	s := newTestSystem(t)
	_, _ = s.CreateBranch("b1", "Branch One")
	order, err := s.CreateOrder("b1", "o1", "Customer", []model.OrderItem{{Name: "Tea", Quantity: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != model.OrderPending {
		t.Fatalf("initial status = %s", order.Status)
	}
	updated, err := s.UpdateOrderStatus("o1", model.OrderPreparing)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != model.OrderPreparing {
		t.Fatalf("updated status = %s", updated.Status)
	}
	if _, err := s.UpdateOrderStatus("o1", model.OrderDelivered); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

func TestBroadcastAndTransferStopsNewOldBranchEvents(t *testing.T) {
	s := newTestSystem(t)
	_, _ = s.CreateBranch("a", "A")
	_, _ = s.CreateBranch("b", "B")
	employee, err := s.RegisterEmployee("e1", "Ali", model.RoleStaff, "a")
	if err != nil {
		t.Fatal(err)
	}

	// Publish an event and wait for its delivery before moving the employee.
	if err := s.publishTestEvent("a", "before"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-employee.Events():
		if got.OrderID != "before" {
			t.Fatalf("unexpected first event: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive event from original branch")
	}

	if err := s.SwitchEmployeeBranch("e1", "b"); err != nil {
		t.Fatal(err)
	}
	if err := s.publishTestEvent("a", "after-old"); err != nil {
		t.Fatal(err)
	}
	if err := s.publishTestEvent("b", "after-new"); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-employee.Events():
		if got.BranchID != "b" || got.OrderID != "after-new" {
			t.Fatalf("employee received unexpected post-transfer event: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive event from new branch")
	}
	select {
	case got := <-employee.Events():
		t.Fatalf("received new event from previous branch: %+v", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func (s *OrderSystem) publishTestEvent(branchID, orderID string) error {
	branch, err := s.GetBranch(branchID)
	if err != nil {
		return err
	}
	return branch.PublishEvent(context.Background(), model.OrderEvent{ID: orderID, Type: model.EventOrderCreated, OrderID: orderID, BranchID: branchID, CreatedAt: time.Now()})
}

func TestRegisterRolesAndDuplicateIDs(t *testing.T) {
	s := newTestSystem(t)
	_, _ = s.CreateBranch("b1", "Branch")
	for _, role := range []model.Role{model.RoleManager, model.RoleCashier, model.RoleChef, model.RoleStaff} {
		id := fmt.Sprintf("%s-employee", role)
		if _, err := s.RegisterEmployee(id, string(role), role, "b1"); err != nil {
			t.Fatalf("register role %s: %v", role, err)
		}
	}
	if _, err := s.RegisterEmployee("staff-employee", "Duplicate", model.RoleStaff, "b1"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestConcurrentOrderUpdatesAndShutdown(t *testing.T) {
	s := newTestSystem(t)
	_, _ = s.CreateBranch("b1", "Branch")
	_, _ = s.RegisterEmployee("e1", "Staff", model.RoleStaff, "b1")
	_, err := s.CreateOrder("b1", "o1", "Customer", []model.OrderItem{{Name: "Soup", Quantity: 1}})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = s.GetOrder("o1")
			}
		}()
	}
	wg.Wait()
	s.Shutdown()
	if _, err := s.CreateOrder("b1", "after", "Customer", []model.OrderItem{{Name: "Soup", Quantity: 1}}); !errors.Is(err, ErrSystemClosed) {
		t.Fatalf("expected closed error, got %v", err)
	}
}
