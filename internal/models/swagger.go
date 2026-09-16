package models

// SwaggerResponse is the standard API response envelope with realistic examples.
type SwaggerResponse struct {
	Success bool        `json:"success" example:"true"`
	Message string      `json:"message" example:"operation successful"`
	Data    interface{} `json:"data,omitempty"`
	Error   *SwaggerError `json:"error,omitempty"`
}

// SwaggerError represents error details.
type SwaggerError struct {
	Code    string `json:"code" example:"VALIDATION_ERROR"`
	Details string `json:"details" example:"field is required"`
}

// SwaggerUser is the user object returned in responses.
type SwaggerUser struct {
	ID          string  `json:"id" example:"2d944ed6-285e-4c13-8911-89520c34e43d"`
	Email       *string `json:"email,omitempty" example:"test@example.com"`
	Username    *string `json:"username,omitempty" example:"johndoe"`
	DisplayName *string `json:"display_name,omitempty" example:"John Doe"`
	CreatedAt   string  `json:"created_at" example:"2026-09-16T08:23:55.441437Z"`
	UpdatedAt   string  `json:"updated_at" example:"2026-09-16T08:23:55.441437Z"`
}

// SwaggerTokenResponse is the response from login/refresh.
type SwaggerTokenResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"login successful"`
	Data    struct {
		AccessToken  string `json:"access_token" example:"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIyZDk0NGVkNi0yODVlLTRjMTMtODkxMS04OTUyMGMzNGU0M2QiLCJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJ1c2VybmFtZSI6ImpvaG5kb2UiLCJpYXQiOjE3ODk1NDcwNTksImV4cCI6MTc4OTU0Nzk1OX0.RWWXCR49EGPyZvVeqDTmKAdn3pnpXXWK1yx9Jk5-BRY"`
		RefreshToken string `json:"refresh_token" example:"54204a5db8e34b88874032e4b63add0e538747c0e498f4db7217b706dda18c24"`
	} `json:"data"`
}

// SwaggerUserResponse is the response from register/me.
type SwaggerUserResponse struct {
	Success bool         `json:"success" example:"true"`
	Message string       `json:"message" example:"user registered"`
	Data    SwaggerUser  `json:"data"`
}

// SwaggerAPIKeyResponse is the response from create API key.
type SwaggerAPIKeyResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"api key created"`
	Data    struct {
		APIKey struct {
			ID        string  `json:"id" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
			Name      string  `json:"name" example:"portfolio-backend"`
			Prefix    string  `json:"prefix" example:"ak_a1b2c3d4"`
			ExpiresAt *string `json:"expires_at,omitempty" example:"2026-12-15T00:00:00Z"`
			CreatedAt string  `json:"created_at" example:"2026-09-16T08:30:00Z"`
		} `json:"api_key"`
		Key string `json:"key" example:"ak_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"`
	} `json:"data"`
}
