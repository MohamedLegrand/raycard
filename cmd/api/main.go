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
	"raycard/internal/application"
	"raycard/internal/infrastructure/auth/jwt"
	"raycard/internal/infrastructure/config"
	"raycard/internal/infrastructure/database/postgres"
	"raycard/internal/infrastructure/notification/brevo"
	apihttp "raycard/internal/transport/http"
	"raycard/internal/transport/http/handlers"
	"raycard/internal/transport/http/middleware"
)

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

	pool, err := postgres.NewPool(connCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("connexion base de données")
	}
	defer pool.Close()

	// Adapters de persistance
	utilisateurRepo := postgres.NewUtilisateurRepository(pool)
	walletRepo := postgres.NewWalletRepository(pool)
	reglesKycRepo := postgres.NewReglesKycRepository(pool)
	refreshTokenRepo := postgres.NewRefreshTokenRepository(pool)
	tokenReinitialisationRepo := postgres.NewTokenReinitialisationRepository(pool)
	ticketConnexionRepo := postgres.NewTicketConnexionRepository(pool)
	dossierKycRepo := postgres.NewDossierKycRepository(pool)
	auditLogRepo := postgres.NewAuditLogRepository(pool)
	txManager := postgres.NewTxManager(pool)

	// Adapters de sécurité et de notification
	tokenGenerator := jwt.NewTokenGenerator(cfg.JWTSecret)
	notifieur := brevo.NewNotifieur(cfg.BrevoAPIKey, cfg.BrevoEmailExpediteur)

	// Use cases (application)
	kycUseCase := application.NewKycService(utilisateurRepo, walletRepo, reglesKycRepo, dossierKycRepo, txManager)
	authUseCase := application.NewAuthService(utilisateurRepo, refreshTokenRepo, tokenReinitialisationRepo, ticketConnexionRepo, tokenGenerator, notifieur, txManager)
	adminKycUseCase := application.NewAdminKycService(utilisateurRepo, dossierKycRepo, auditLogRepo, txManager)

	// Transport HTTP
	validate := validator.New()
	kycHandler := handlers.NewKycHandler(kycUseCase, validate)
	authHandler := handlers.NewAuthHandler(authUseCase, validate)
	adminKycHandler := handlers.NewAdminKycHandler(adminKycUseCase, validate)

	app := fiber.New(fiber.Config{
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
