package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	APIKey      string
	BcryptCost  int
}

func Load() *Config {
	cost := 12
	if v := os.Getenv("BCRYPT_COST"); v != "" {
		cost = atoi(v)
	}
	return &Config{
		Port:        getenv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		APIKey:      os.Getenv("API_KEY"),
		BcryptCost:  cost,
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
