package service

import "errors"

var (
	ErrSystemClosed        = errors.New("order system is shutting down or already closed")
	ErrInvalidArgument     = errors.New("invalid argument")
	ErrAlreadyExists       = errors.New("resource already exists")
	ErrNotFound            = errors.New("resource not found")
	ErrInvalidTransition   = errors.New("invalid order status transition")
	ErrSameBranch          = errors.New("employee is already assigned to this branch")
	ErrEmployeeNotAssigned = errors.New("employee is not assigned to a branch")
)
