package model

import "time"

type EventType string

const (
	EventOrderCreated       EventType = "ORDER_CREATED"
	EventOrderStatusChanged EventType = "ORDER_STATUS_CHANGED"
	EventOrderCancelled     EventType = "ORDER_CANCELLED"
)

type OrderEvent struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	OrderID   string    `json:"order_id"`
	BranchID  string    `json:"branch_id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
