package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	_ "raycard/docs" // docs générés par `swag init`, nécessaires pour servir la spec Swagger
	appauth "raycard/internal/application/auth"
	appkyc "raycard/internal/application/kyc"
	"raycard/internal/infrastructure/auth/google"
	"raycard/internal/infrastructure/auth/jwt"
	"raycard/internal/infrastructure/config"
	pgauth "raycard/internal/infrastructure/database/postgres/auth"
	pgcommun "raycard/internal/infrastructure/database/postgres/commun"
	pgkyc "raycard/internal/infrastructure/database/postgres/kyc"
	"raycard/internal/infrastructure/notification/brevo"
	"raycard/internal/infrastructure/ocr/tesseract"
	"raycard/internal/infrastructure/storage/local"
	apihttp "raycard/internal/transport/http"
	handlersauth "raycard/internal/transport/http/handlers/auth"
	handlerskyc "raycard/internal/transport/http/handlers/kyc"
	"raycard/internal/transport/http/middleware"
)

// tailleMaxCorpsRequete autorise l'upload de photos de documents KYC
// (plusieurs Mo depuis un téléphone), au-delà de la limite par défaut
// de Fiber (4 Mo).
const tailleMaxCorpsRequete = 10 * 1024 * 1024

// @title						RAYCARD API
// @version					1.0
// @description				API backend de la plateforme RAYCARD (wallet Mobile Money + cartes Visa virtuelles).
// @contact.name				RAYCARD — HR-Skills SARL
// @host						localhost:3000
// @BasePath					/api/v1
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("chargement configuration")
	}

	connCtx, cancelConn := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelConn()

	pool, err := pgcommun.NewPool(connCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("connexion base de données")
	}
	defer pool.Close()

	// Adapters de persistance
	utilisateurRepo := pgcommun.NewUtilisateurRepository(pool)
	walletRepo := pgcommun.NewWalletRepository(pool)
	reglesKycRepo := pgcommun.NewReglesKycRepository(pool)
	refreshTokenRepo := pgauth.NewRefreshTokenRepository(pool)
	tokenReinitialisationRepo := pgauth.NewTokenReinitialisationRepository(pool)
	ticketConnexionRepo := pgauth.NewTicketConnexionRepository(pool)
	cleAppareilRepo := pgauth.NewCleAppareilRepository(pool)
	challengeEmpreinteRepo := pgauth.NewChallengeEmpreinteRepository(pool)
	dossierKycRepo := pgkyc.NewDossierKycRepository(pool)
	documentKycRepo := pgkyc.NewDocumentKycRepository(pool)
	auditLogRepo := pgcommun.NewAuditLogRepository(pool)
	txManager := pgcommun.NewTxManager(pool)

	// Adapters de sécurité et de notification
	tokenGenerator := jwt.NewTokenGenerator(cfg.JWTSecret)
	notifieur := brevo.NewNotifieur(cfg.BrevoAPIKey, cfg.BrevoEmailExpediteur)
	googleAuthProvider := google.NewVerificateurToken(cfg.GoogleClientID)

	// Adapters OCR (documents KYC) : stockage sur disque local et
	// extraction du texte via le binaire tesseract.
	stockageDocuments := local.NewStockageFichier(cfg.UploadsDir)
	ocrExtracteur := tesseract.NewExtracteur(cfg.TesseractLang)

	// Use cases (application)
	kycUseCase := appkyc.NewKycService(
		utilisateurRepo, walletRepo, reglesKycRepo, dossierKycRepo, documentKycRepo,
		stockageDocuments, ocrExtracteur, txManager,
	)
	authUseCase := appauth.NewAuthService(
		utilisateurRepo, walletRepo, reglesKycRepo, refreshTokenRepo, tokenReinitialisationRepo, ticketConnexionRepo,
		cleAppareilRepo, challengeEmpreinteRepo,
		tokenGenerator, notifieur, googleAuthProvider, txManager,
	)
	adminKycUseCase := appkyc.NewAdminKycService(utilisateurRepo, dossierKycRepo, documentKycRepo, auditLogRepo, txManager)

	// Transport HTTP
	validate := validator.New()
	kycHandler := handlerskyc.NewKycHandler(kycUseCase, validate)
	authHandler := handlersauth.NewAuthHandler(authUseCase, validate)
	adminKycHandler := handlerskyc.NewAdminKycHandler(adminKycUseCase, validate)

	app := fiber.New(fiber.Config{
		BodyLimit: tailleMaxCorpsRequete,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"erreur": err.Error()})
		},
	})
	app.Use(middleware.Logger(logger))

	apihttp.SetupRoutes(app, apihttp.Handlers{Kyc: kycHandler, Auth: authHandler, AdminKyc: adminKycHandler}, tokenGenerator)

	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			logger.Fatal().Err(err).Msg("démarrage serveur")
		}
	}()
	logger.Info().Str("port", cfg.Port).Msg("serveur démarré")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("arrêt du serveur en cours")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("erreur pendant l'arrêt")
	}
}
