package model

import "testing"

func TestOrderStatusTransitions(t *testing.T) {
	valid := []struct{ from, to OrderStatus }{
		{OrderPending, OrderPreparing}, {OrderPending, OrderCancelled},
		{OrderPreparing, OrderReady}, {OrderPreparing, OrderCancelled},
		{OrderReady, OrderDelivered},
	}
	for _, tt := range valid {
		if !tt.from.CanTransitionTo(tt.to) {
			t.Errorf("expected %s -> %s to be valid", tt.from, tt.to)
		}
	}
	invalid := []struct{ from, to OrderStatus }{
		{OrderPending, OrderReady}, {OrderPreparing, OrderDelivered},
		{OrderReady, OrderPreparing}, {OrderDelivered, OrderCancelled},
		{OrderCancelled, OrderPending},
	}
	for _, tt := range invalid {
		if tt.from.CanTransitionTo(tt.to) {
			t.Errorf("expected %s -> %s to be invalid", tt.from, tt.to)
		}
	}
}
