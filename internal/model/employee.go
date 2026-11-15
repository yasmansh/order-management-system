package model

type EmployeeInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Role     Role   `json:"role"`
	BranchID string `json:"branch_id"`
}
