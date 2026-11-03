package models

// LoginRequest is the body of POST /auth/login - username + password only.
// No badge number and no OTP step: panel accounts are not members.
type LoginRequest struct {
	Username string `json:"username" validate:"required,max=50"`
	Password string `json:"password" validate:"required,max=128"`
}
