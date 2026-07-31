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
	Port                 string
	DatabaseURL          string
	JWTSecret            string
	BrevoAPIKey          string
	BrevoEmailExpediteur string
	GoogleClientID       string
	Env                  string
	UploadsDir           string
	TesseractLang        string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // absence de .env attendue en production

	cfg := &Config{
		Port:                 getEnv("PORT", "3000"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		JWTSecret:            os.Getenv("JWT_SECRET"),
		BrevoAPIKey:          os.Getenv("BREVO_API_KEY"),
		BrevoEmailExpediteur: os.Getenv("BREVO_EMAIL_EXPEDITEUR"),
		GoogleClientID:       os.Getenv("GOOGLE_CLIENT_ID"),
		Env:                  getEnv("APP_ENV", "development"),
		UploadsDir:           getEnv("UPLOADS_DIR", "./uploads"),
		TesseractLang:        getEnv("TESSERACT_LANG", "fra"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL est requis")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET est requis")
	}
	if cfg.BrevoAPIKey == "" {
		return nil, fmt.Errorf("BREVO_API_KEY est requis")
	}
	if cfg.BrevoEmailExpediteur == "" {
		return nil, fmt.Errorf("BREVO_EMAIL_EXPEDITEUR est requis")
	}
	if cfg.GoogleClientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID est requis")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
