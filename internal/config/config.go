package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	APIKey      string
	BcryptCost  int

	// Resend for password-reset mail. An empty APIKey disables sending: the
	// endpoint still reports success (it must not reveal whether an address
	// exists) and logs that no mail went out.
	ResendAPIKey    string
	ResendFromEmail string
	ResendFromName  string
	// FrontendURL builds the reset link, e.g. https://cafe.abuamar.online.
	// Empty sends the raw token in the body instead — what a dev box wants.
	FrontendURL string
}

func Load() *Config {
	cost := 12
	if v := os.Getenv("BCRYPT_COST"); v != "" {
		cost = atoi(v)
	}
	return &Config{
		Port:            getenv("PORT", "8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		APIKey:          os.Getenv("API_KEY"),
		BcryptCost:      cost,
		ResendAPIKey:    os.Getenv("RESEND_API_KEY"),
		ResendFromEmail: getenv("RESEND_FROM_EMAIL", "noreply@abuamar.online"),
		ResendFromName:  getenv("RESEND_FROM_NAME", "Abu Amar"),
		FrontendURL:     os.Getenv("FRONTEND_URL"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	if n == 0 {
		return 12
	}
	return n
}
