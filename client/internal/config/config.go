package config

import (
	"os"
)

// Config holds application configuration
type Config struct {
	DatabaseURL        string
	ExternalServiceURL string
	RedisURL           string
	Port               string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://scooter_user:scooter_password@localhost:5432/client-database?sslmode=disable"),
		ExternalServiceURL: getEnv("EXTERNAL_SERVICE_URL", "http://localhost:8081"),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379/0"),
		Port:               getEnv("PORT", "8080"),
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
