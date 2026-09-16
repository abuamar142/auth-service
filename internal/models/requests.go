package models

// RegisterRequest is the request body for POST /auth/register.
type RegisterRequest struct {
	Email       string `json:"email" example:"test@example.com"`
	Username    string `json:"username" example:"johndoe"`
	Password    string `json:"password" example:"secretpass123"`
	DisplayName string `json:"display_name" example:"John Doe"`
}

// LoginRequest is the request body for POST /auth/login.
type LoginRequest struct {
	Identifier string `json:"identifier" example:"test@example.com"`
	Password   string `json:"password" example:"secretpass123"`
}

// RefreshRequest is the request body for POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" example:"e1d2a3b4c5f6a7b8c9d0e1f2a3b4c5d6a7b8c9d0e1f2a3b4c5d6a7b8c9d0e1f2"`
}

// LogoutRequest is the request body for POST /auth/logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" example:"e1d2a3b4c5f6a7b8c9d0e1f2a3b4c5d6a7b8c9d0e1f2a3b4c5d6a7b8c9d0e1f2"`
}

// CreateAPIKeyRequest is the request body for POST /api-keys.
type CreateAPIKeyRequest struct {
	Name      string `json:"name" example:"portfolio-backend"`
	ExpiresIn string `json:"expires_in,omitempty" example:"90d"`
}
