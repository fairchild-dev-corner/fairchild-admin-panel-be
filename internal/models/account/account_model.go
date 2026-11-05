package models

// Account matches Account in the admin panel (src/models/admin.ts): the
// signed-in panel user's own `users_panel` row, their active sessions,
// recent sign-in attempts and recent actions. Times are RFC 3339.
type Account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Email       string `json:"email,omitempty"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	LastLoginAt string `json:"lastLoginAt,omitempty"`

	Sessions      []AccountSession  `json:"sessions"`
	SignIns       []AccountSignIn   `json:"signIns"`
	Activity      []AccountActivity `json:"activity"`
	ActivityCount int               `json:"activityCount"`
}

// AccountSession is one unexpired, unrevoked users_panel_sessions row - a
// browser that's still signed in.
type AccountSession struct {
	IPAddress string `json:"ipAddress,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt"`
}

// AccountSignIn is one users_panel_login_audit row for this user, failed
// attempts on their username included.
type AccountSignIn struct {
	Success   bool   `json:"success"`
	Reason    string `json:"reason,omitempty"`
	IPAddress string `json:"ipAddress,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	At        string `json:"at"`
}

// AccountActivity is one admin_audit_logs row this user wrote.
type AccountActivity struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Target string `json:"target"`
	At     string `json:"at"`
}

// UpdateProfileRequest is the body of PUT /admin/account/profile. Email may
// be left empty. The current password confirms the change.
type UpdateProfileRequest struct {
	FirstName       string `json:"firstName" validate:"required,max=100"`
	LastName        string `json:"lastName" validate:"required,max=100"`
	Email           string `json:"email" validate:"omitempty,email,max=255"`
	CurrentPassword string `json:"currentPassword" validate:"required"`
}

// ChangePasswordRequest is the body of PUT /admin/account/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required"`
}
