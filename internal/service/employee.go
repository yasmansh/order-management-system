package service

import (
	"sync"

	"order-management/internal/model"
)

type Employee struct {
	id   string
	name string
	role model.Role

	// mu protects currentBranch. SwitchEmployeeBranch takes this lock before
	// locking branch membership maps, establishing one global lock order.
	mu            sync.RWMutex
	currentBranch *Branch
	events        chan model.OrderEvent
}

func (e *Employee) Info() model.EmployeeInfo {
	if e == nil {
		return model.EmployeeInfo{}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	branchID := ""
	if e.currentBranch != nil {
		branchID = e.currentBranch.id
	}
	return model.EmployeeInfo{ID: e.id, Name: e.name, Role: e.role, BranchID: branchID}
}

func (e *Employee) Events() <-chan model.OrderEvent {
	if e == nil {
		return nil
	}
	return e.events
}
