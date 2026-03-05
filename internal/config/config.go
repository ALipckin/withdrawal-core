package config

import (
	"fmt"
	"os"
)

type Config struct {
	AppPort         string
	DBUser          string
	DBPassword      string
	DBHost          string
	DBPort          string
	DBName          string
	AuthBearerToken string
	MigrationsPath  string
}

func Load() (*Config, error) {
	cfg := &Config{
		AppPort:         getEnv("APP_PORT", "8080"),
		DBUser:          os.Getenv("DB_USER"),
		DBPassword:      os.Getenv("DB_PASSWORD"),
		DBHost:          getEnv("DB_HOST", "db"),
		DBPort:          getEnv("DB_PORT", "5432"),
		DBName:          os.Getenv("DB_DATABASE"),
		AuthBearerToken: os.Getenv("AUTH_BEARER_TOKEN"),
		MigrationsPath:  getEnv("MIGRATIONS_PATH", "file://internal/database/migrations"),
	}

	if cfg.DBUser == "" || cfg.DBPassword == "" || cfg.DBName == "" {
		return nil, fmt.Errorf("database env vars are required: DB_USER, DB_PASSWORD, DB_DATABASE")
	}
	if cfg.AuthBearerToken == "" {
		return nil, fmt.Errorf("AUTH_BEARER_TOKEN is required")
	}

	return cfg, nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

func getEnv(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
