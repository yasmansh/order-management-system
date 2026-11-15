package model

type Role string

const (
	RoleManager Role = "manager"
	RoleCashier Role = "cashier"
	RoleChef    Role = "chef"
	RoleStaff   Role = "staff"
)

func (r Role) IsValid() bool {
	switch r {
	case RoleManager, RoleCashier, RoleChef, RoleStaff:
		return true
	default:
		return false
	}
}
