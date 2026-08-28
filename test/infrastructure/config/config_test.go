package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/infrastructure/config"
)

// definirEnvValide place toutes les variables requises par config.Load
// à des valeurs valides, sauf JWT_SECRET (paramétrable par chaque test)
// — évite de répéter les mêmes 6 champs "requis" dans chaque cas.
func definirEnvValide(t *testing.T, jwtSecret string) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/raycard?sslmode=disable")
	t.Setenv("JWT_SECRET", jwtSecret)
	t.Setenv("BREVO_API_KEY", "test-key")
	t.Setenv("BREVO_EMAIL_EXPEDITEUR", "no-reply@raycard.app")
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("HRPAY_PUBLIC_KEY", "test-public")
	t.Setenv("HRPAY_SECRET_KEY", "test-secret")
	t.Setenv("HRPAY_WEBHOOK_SECRET", "test-webhook")
}

func TestLoad_RefuseSecretVide(t *testing.T) {
	definirEnvValide(t, "")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET est requis")
}

func TestLoad_RefusePlaceholderExemple(t *testing.T) {
	// Exactement la valeur committée dans .env.example — visible par
	// quiconque a accès au repo, jamais utilisable telle quelle.
	definirEnvValide(t, "change-moi-en-production")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "valeur d'exemple")
}

func TestLoad_RefuseSecretTropCourt(t *testing.T) {
	definirEnvValide(t, "trop-court")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "32 caractères")
}

func TestLoad_AccepteSecretValide(t *testing.T) {
	definirEnvValide(t, "un-secret-suffisamment-long-et-different-du-placeholder")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.JWTSecret)
}
