// Package config charge la configuration de l'application depuis les
// variables d'environnement (godotenv charge un fichier .env en
// développement uniquement ; en production les variables sont injectées
// par la plateforme).
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	Env         string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // absence de .env attendue en production

	cfg := &Config{
		Port:        getEnv("PORT", "3000"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		Env:         getEnv("APP_ENV", "development"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL est requis")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET est requis")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
