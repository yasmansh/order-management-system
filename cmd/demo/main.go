package main

import (
	"context"
	"fmt"
	"time"

	"order-management/internal/model"
	"order-management/internal/service"
)

func main() {
	system, err := service.NewOrderSystem(context.Background())
	must(err)
	defer system.Shutdown()

	_, err = system.CreateBranch("branch-1", "Central")
	must(err)
	_, err = system.CreateBranch("branch-2", "North")
	must(err)

	ali, err := system.RegisterEmployee("emp-1", "Ali", model.RoleStaff, "branch-1")
	must(err)
	_, err = system.RegisterEmployee("emp-2", "Sara", model.RoleChef, "branch-1")
	must(err)
	_, err = system.RegisterEmployee("emp-3", "Reza", model.RoleManager, "branch-2")
	must(err)

	consume := func(name string, events <-chan model.OrderEvent, done <-chan struct{}) {
		go func() {
			for {
				select {
				case <-done:
					return
				case event := <-events:
					fmt.Printf("[%s] branch=%s type=%s order=%s message=%q\n", name, event.BranchID, event.Type, event.OrderID, event.Message)
				}
			}
		}()
	}
	done := make(chan struct{})
	defer close(done)
	consume("Ali", ali.Events(), done)

	_, err = system.CreateOrder("branch-1", "order-100", "Customer One", []model.OrderItem{{Name: "Burger", Quantity: 2}})
	must(err)
	time.Sleep(50 * time.Millisecond)

	must(system.SwitchEmployeeBranch("emp-1", "branch-2"))
	_, err = system.CreateOrder("branch-1", "order-101", "Customer Two", []model.OrderItem{{Name: "Salad", Quantity: 1}})
	must(err)
	_, err = system.CreateOrder("branch-2", "order-102", "Customer Three", []model.OrderItem{{Name: "Pizza", Quantity: 1}})
	must(err)

	time.Sleep(100 * time.Millisecond)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}