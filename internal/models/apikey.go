package models

import "time"

type APIKey struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	KeyHash   string     `json:"-"`
	Prefix    string     `json:"prefix"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
