package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        *string   `json:"email,omitempty"`
	Username     *string   `json:"username,omitempty"`
	PasswordHash string    `json:"-"`
	DisplayName  *string   `json:"display_name,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
