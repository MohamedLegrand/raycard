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

	// S3Bucket vide (défaut) = stockage local sur disque (voir
	// local.StockageFichier), suffisant en développement sans dépendance
	// externe. Renseigné = bascule sur le stockage objet compatible S3
	// (voir s3.StockageFichier et cmd/api/main.go, qui choisit l'une ou
	// l'autre implémentation selon ce champ) — recommandé en production,
	// le disque local ne survivant ni à un redéploiement ni à plusieurs
	// instances.
	S3Bucket          string
	S3Region          string
	S3Endpoint        string // vide = AWS S3 natif ; renseigné = tout fournisseur S3-compatible (R2, B2, MinIO...)
	S3AccessKeyID     string
	S3SecretAccessKey string
	// S3UsePathStyle : vrai pour la plupart des fournisseurs non-AWS (voir
	// leur documentation respective), faux pour AWS S3 natif.
	S3UsePathStyle     bool
	HrPayPublicKey     string
	HrPaySecretKey     string
	HrPayWebhookSecret string

	// CorsAllowedOrigins : liste d'origines autorisées séparées par des
	// virgules (ex: "https://backoffice.raycard.io"). Vide par défaut —
	// aucune origine n'est autorisée tant que ce n'est pas configuré
	// explicitement (voir cmd/api/main.go, qui n'active le middleware CORS
	// que si cette valeur est non vide). L'app mobile n'appelle jamais
	// l'API depuis un navigateur, donc n'a besoin d'aucune origine ; ce
	// réglage ne sert qu'à un futur front web (ex: back-office).
	CorsAllowedOrigins string
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
		S3Bucket:             os.Getenv("S3_BUCKET"),
		S3Region:             os.Getenv("S3_REGION"),
		S3Endpoint:           os.Getenv("S3_ENDPOINT"),
		S3AccessKeyID:        os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretAccessKey:    os.Getenv("S3_SECRET_ACCESS_KEY"),
		S3UsePathStyle:       getEnv("S3_USE_PATH_STYLE", "true") == "true",
		HrPayPublicKey:       os.Getenv("HRPAY_PUBLIC_KEY"),
		HrPaySecretKey:       os.Getenv("HRPAY_SECRET_KEY"),
		HrPayWebhookSecret:   os.Getenv("HRPAY_WEBHOOK_SECRET"),
		CorsAllowedOrigins:   os.Getenv("CORS_ALLOWED_ORIGINS"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL est requis")
	}
	if err := validerJWTSecret(cfg.JWTSecret); err != nil {
		return nil, err
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
	if cfg.HrPayPublicKey == "" {
		return nil, fmt.Errorf("HRPAY_PUBLIC_KEY est requis")
	}
	if cfg.HrPaySecretKey == "" {
		return nil, fmt.Errorf("HRPAY_SECRET_KEY est requis")
	}
	if cfg.HrPayWebhookSecret == "" {
		return nil, fmt.Errorf("HRPAY_WEBHOOK_SECRET est requis")
	}
	// S3_BUCKET reste optionnel (voir son commentaire sur Config) : ne
	// valider la présence des autres champs S3 que si le stockage objet
	// est effectivement activé — jamais bloquer le démarrage en
	// développement, où le stockage local par défaut ne les demande pas.
	if cfg.S3Bucket != "" {
		if cfg.S3Region == "" {
			return nil, fmt.Errorf("S3_REGION est requis quand S3_BUCKET est renseigné")
		}
		if cfg.S3AccessKeyID == "" {
			return nil, fmt.Errorf("S3_ACCESS_KEY_ID est requis quand S3_BUCKET est renseigné")
		}
		if cfg.S3SecretAccessKey == "" {
			return nil, fmt.Errorf("S3_SECRET_ACCESS_KEY est requis quand S3_BUCKET est renseigné")
		}
	}

	return cfg, nil
}

// dureeMinJWTSecret : sous cette longueur, un secret HS256 est trop
// facile à retrouver par force brute — 32 caractères donne au moins
// 256 bits d'entropie si le secret est réellement aléatoire (voir
// recommandation RFC 7518 §3.2 : la clé HMAC doit faire au moins la
// taille de sortie du hash, 256 bits pour HS256).
const longueurMinJWTSecret = 32

// jwtSecretPlaceholder est la valeur d'exemple de .env.example, commitée
// dans le repo et donc connue de quiconque y a accès. La refuser
// explicitement évite le scénario "copie de .env.example en prod sans
// changer le secret" : sans ce garde-fou, un JWT_SECRET non vide mais
// public permettrait de forger un token valide pour n'importe quel
// utilisateur, y compris un rôle admin (RequireAdmin ne fait confiance
// qu'à la signature).
const jwtSecretPlaceholder = "change-moi-en-production"

func validerJWTSecret(secret string) error {
	if secret == "" {
		return fmt.Errorf("JWT_SECRET est requis")
	}
	if secret == jwtSecretPlaceholder {
		return fmt.Errorf("JWT_SECRET ne peut pas être la valeur d'exemple de .env.example — génère un secret réel (ex: openssl rand -base64 32)")
	}
	if len(secret) < longueurMinJWTSecret {
		return fmt.Errorf("JWT_SECRET doit faire au moins %d caractères", longueurMinJWTSecret)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
