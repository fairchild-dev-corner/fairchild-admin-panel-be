package models

// PanelUser matches PanelUser in the admin panel (src/models/admin.ts): one
// non-deleted users_panel account. Times are RFC 3339.
type PanelUser struct {
	InternalID  int64  `json:"-"`
	ID          string `json:"id"`
	Username    string `json:"username"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Email       string `json:"email,omitempty"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	LastLoginAt string `json:"lastLoginAt,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

// CreatePanelUserRequest is the body of POST /admin/users. The admin sets
// the first password and passes it on; the new user can change it on their
// Account page. A manager, supervisor or staff account starts with its
// role's preset capabilities.
type CreatePanelUserRequest struct {
	Username  string `json:"username" validate:"required,max=50"`
	FirstName string `json:"firstName" validate:"required,max=100"`
	LastName  string `json:"lastName" validate:"required,max=100"`
	Email     string `json:"email" validate:"omitempty,email,max=255"`
	Role      string `json:"role" validate:"required,oneof=super_admin admin manager supervisor staff"`
	Password  string `json:"password" validate:"required,max=128"`
}

// UpdateRoleRequest is the body of PUT /admin/users/:id/role. The account
// keeps its assigned capabilities; they matter only below admin.
type UpdateRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=super_admin admin manager supervisor staff"`
}

// UpdateStatusRequest is the body of PUT /admin/users/:id/status.
// Suspending signs the account out everywhere.
type UpdateStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=active suspended"`
}
