package model

import "time"

type OrderStatus string

const (
	OrderPending   OrderStatus = "pending"
	OrderPreparing OrderStatus = "preparing"
	OrderReady     OrderStatus = "ready"
	OrderDelivered OrderStatus = "delivered"
	OrderCancelled OrderStatus = "cancelled"
)

// CanTransitionTo contains the complete state-machine rule for an order.
// Keeping this validation separate makes the transition policy easy to test
// and prevents service methods from duplicating business rules.
func (s OrderStatus) CanTransitionTo(next OrderStatus) bool {
	switch s {
	case OrderPending:
		return next == OrderPreparing || next == OrderCancelled
	case OrderPreparing:
		return next == OrderReady || next == OrderCancelled
	case OrderReady:
		return next == OrderDelivered
	default: // Delivered and Cancelled are terminal states.
		return false
	}
}

type OrderItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type Order struct {
	ID        string      `json:"id"`
	BranchID  string      `json:"branch_id"`
	Customer  string      `json:"customer"`
	Items     []OrderItem `json:"items"`
	Status    OrderStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// Clone returns a defensive copy so callers cannot mutate an order stored
// in memory without going through the service's synchronization boundary.
func (o Order) Clone() Order {
	copyOfOrder := o
	copyOfOrder.Items = append([]OrderItem(nil), o.Items...)
	return copyOfOrder
}
