package models

// UserPanelResponse matches UserResponseDto in the admin panel
// (src/models/auth.ts).
type UserPanelResponse struct {
	ID        int64  `json:"id"`
	UUID      string `json:"uuid"`
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

func NewUserPanelResponse(u *UserPanel) UserPanelResponse {
	return UserPanelResponse{
		ID:        u.ID,
		UUID:      u.UUID,
		Username:  u.Username,
		Email:     u.Email.String,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Role:      u.Role,
	}
}

// AuthTokenResponse is returned by /auth/login and /auth/refresh. The
// refresh token is never in the body - it goes out as an HttpOnly cookie.
type AuthTokenResponse struct {
	AccessToken string            `json:"access_token"`
	TokenType   string            `json:"token_type"`
	ExpiresIn   int64             `json:"expires_in"`
	User        UserPanelResponse `json:"user"`
}

// IssuedTokens is what AuthService hands back to the handler: the response
// body plus the raw refresh token for the cookie.
type IssuedTokens struct {
	Response     AuthTokenResponse
	RefreshToken string
	RefreshTTL   int64 // seconds
}
