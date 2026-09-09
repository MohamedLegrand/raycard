package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/rs/zerolog"

	_ "raycard/docs" // docs générés par `swag init`, nécessaires pour servir la spec Swagger
	appadmin "raycard/internal/application/admin"
	appauth "raycard/internal/application/auth"
	appcarte "raycard/internal/application/carte"
	appkyc "raycard/internal/application/kyc"
	appwallet "raycard/internal/application/wallet"
	inputcarte "raycard/internal/core/ports/input/carte"
	inputwallet "raycard/internal/core/ports/input/wallet"
	outputcommun "raycard/internal/core/ports/output/commun"
	"raycard/internal/infrastructure/auth/google"
	"raycard/internal/infrastructure/auth/jwt"
	"raycard/internal/infrastructure/config"
	pgauth "raycard/internal/infrastructure/database/postgres/auth"
	pgcarte "raycard/internal/infrastructure/database/postgres/carte"
	pgcommun "raycard/internal/infrastructure/database/postgres/commun"
	pgkyc "raycard/internal/infrastructure/database/postgres/kyc"
	pgwallet "raycard/internal/infrastructure/database/postgres/wallet"
	"raycard/internal/infrastructure/notification/brevo"
	"raycard/internal/infrastructure/ocr/tesseract"
	"raycard/internal/infrastructure/paiement/hrpay"
	"raycard/internal/infrastructure/storage/local"
	"raycard/internal/infrastructure/storage/s3"
	apihttp "raycard/internal/transport/http"
	handlersadmin "raycard/internal/transport/http/handlers/admin"
	handlersauth "raycard/internal/transport/http/handlers/auth"
	handlerscarte "raycard/internal/transport/http/handlers/carte"
	handlerskyc "raycard/internal/transport/http/handlers/kyc"
	handlerswallet "raycard/internal/transport/http/handlers/wallet"
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
	var output io.Writer = os.Stdout
	if os.Getenv("APP_ENV") != "production" {
		output = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	}
	logger := zerolog.New(output).With().Timestamp().Logger()

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
	tokenChangementEmailRepo := pgauth.NewTokenChangementEmailRepository(pool)
	verrouConnexionRepo := pgauth.NewVerrouConnexionRepository(pool)
	verrouReinitialisationRepo := pgauth.NewVerrouReinitialisationRepository(pool)
	dossierKycRepo := pgkyc.NewDossierKycRepository(pool)
	documentKycRepo := pgkyc.NewDocumentKycRepository(pool)
	transactionWalletRepo := pgwallet.NewTransactionRepository(pool)
	carteRepo := pgcarte.NewCarteRepository(pool)
	depenseCarteRepo := pgcarte.NewDepenseCarteRepository(pool)
	cardCustomerRepo := pgcarte.NewCardCustomerRepository(pool)
	auditLogRepo := pgcommun.NewAuditLogRepository(pool)
	txManager := pgcommun.NewTxManager(pool)

	// Adapters de sécurité et de notification
	tokenGenerator := jwt.NewTokenGenerator(cfg.JWTSecret)
	notifieur := brevo.NewNotifieur(cfg.BrevoAPIKey, cfg.BrevoEmailExpediteur)
	googleAuthProvider := google.NewVerificateurToken(cfg.GoogleClientID)

	// Agrégateur de paiement (recharge Mobile Money, plus tard cartes
	// virtuelles). Sandbox ou production selon le préfixe des clés, jamais
	// l'URL (voir internal/infrastructure/paiement/hrpay).
	agregateurPaiement, err := hrpay.NewAdapter(cfg.HrPayPublicKey, cfg.HrPaySecretKey, cfg.HrPayWebhookSecret)
	if err != nil {
		logger.Fatal().Err(err).Msg("initialisation agrégateur de paiement")
	}

	// Stockage des fichiers téléversés (documents KYC, photos de profil) :
	// objet compatible S3 si configuré (voir Config.S3Bucket — recommandé
	// en production, seule option qui survit à un redéploiement ou à
	// plusieurs instances), sinon disque local du serveur (suffisant en
	// développement, sans dépendance externe). Le port
	// outputcommun.StockageFichier isole ce choix du reste du code.
	var stockageFichiers outputcommun.StockageFichier
	if cfg.S3Bucket != "" {
		stockageFichiers = s3.NewStockageFichier(
			cfg.S3Bucket, cfg.S3Region, cfg.S3Endpoint, cfg.S3AccessKeyID, cfg.S3SecretAccessKey, cfg.S3UsePathStyle,
		)
		logger.Info().Str("bucket", cfg.S3Bucket).Msg("stockage fichiers : objet (S3-compatible)")
	} else {
		stockageFichiers = local.NewStockageFichier(cfg.UploadsDir)
		logger.Info().Str("répertoire", cfg.UploadsDir).Msg("stockage fichiers : disque local")
	}
	ocrExtracteur := tesseract.NewExtracteur(cfg.TesseractLang)

	// Use cases (application)
	kycUseCase := appkyc.NewKycService(
		utilisateurRepo, walletRepo, reglesKycRepo, dossierKycRepo, documentKycRepo,
		stockageFichiers, ocrExtracteur, txManager,
	)
	authUseCase := appauth.NewAuthService(
		utilisateurRepo, walletRepo, reglesKycRepo, refreshTokenRepo, tokenReinitialisationRepo, ticketConnexionRepo,
		cleAppareilRepo, challengeEmpreinteRepo, tokenChangementEmailRepo, verrouConnexionRepo, verrouReinitialisationRepo, stockageFichiers,
		tokenGenerator, notifieur, googleAuthProvider, txManager,
	)
	adminKycUseCase := appkyc.NewAdminKycService(utilisateurRepo, dossierKycRepo, documentKycRepo, stockageFichiers, auditLogRepo, txManager)
	walletUseCase := appwallet.NewWalletService(utilisateurRepo, walletRepo, transactionWalletRepo, agregateurPaiement, notifieur, auditLogRepo, txManager)
	carteUseCase := appcarte.NewCarteService(
		utilisateurRepo, walletRepo, transactionWalletRepo, carteRepo, depenseCarteRepo, cardCustomerRepo,
		dossierKycRepo, documentKycRepo, stockageFichiers,
		agregateurPaiement, notifieur, auditLogRepo, txManager,
	)
	adminUseCase := appadmin.NewAdminService(utilisateurRepo, walletRepo, carteRepo, auditLogRepo)

	// walletUseCase et carteUseCase implémentent chacun deux interfaces
	// (client et back-office, voir walletService/carteService) : leur type
	// statique ne porte que l'interface client, d'où cette assertion pour
	// récupérer le second visage back-office du même service.
	adminWalletUseCase := walletUseCase.(inputwallet.AdminWalletUseCase)
	adminCarteUseCase := carteUseCase.(inputcarte.AdminCarteUseCase)

	// Transport HTTP
	validate := validator.New()
	kycHandler := handlerskyc.NewKycHandler(kycUseCase, validate)
	authHandler := handlersauth.NewAuthHandler(authUseCase, validate)
	adminKycHandler := handlerskyc.NewAdminKycHandler(adminKycUseCase, validate)
	walletHandler := handlerswallet.NewWalletHandler(walletUseCase, validate)
	carteHandler := handlerscarte.NewCarteHandler(carteUseCase, validate)
	adminHandler := handlersadmin.NewAdminHandler(adminUseCase, validate)
	adminWalletHandler := handlerswallet.NewAdminWalletHandler(adminWalletUseCase)
	adminCarteHandler := handlerscarte.NewAdminCarteHandler(adminCarteUseCase, validate)

	// Configuration commune à toute instance de l'API (limite de corps,
	// forme JSON des erreurs, récupération de panique) — partagée avec les
	// tests HTTP de la couche transport, voir apihttp.NouvelleApp.
	app := apihttp.NouvelleApp()
	app.Use(middleware.Logger(logger))

	// Désactivé par défaut (voir config.Config.CorsAllowedOrigins) : l'app
	// mobile n'appelle jamais l'API depuis un navigateur, donc n'a besoin
	// d'aucune origine autorisée. Sans ce middleware, un navigateur bloque
	// déjà toute requête cross-origin — c'est le comportement sécurisé par
	// défaut, pas une lacune à combler tant qu'aucun front web n'existe.
	if cfg.CorsAllowedOrigins != "" {
		app.Use(cors.New(cors.Config{
			AllowOrigins: cfg.CorsAllowedOrigins,
			AllowHeaders: "Origin, Content-Type, Accept, Authorization",
			AllowMethods: "GET, POST, PUT, DELETE",
		}))
	}

	apihttp.SetupRoutes(app, apihttp.Handlers{
		Kyc: kycHandler, Auth: authHandler, AdminKyc: adminKycHandler, Wallet: walletHandler, Carte: carteHandler,
		Admin: adminHandler, AdminWallet: adminWalletHandler, AdminCarte: adminCarteHandler,
	}, tokenGenerator)

	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			logger.Fatal().Err(err).Msg("démarrage serveur")
		}
	}()
	logger.Info().Str("port", cfg.Port).Msg("serveur démarré")

	// Job planifié : bascule les recharges dont le délai de retenue de
	// l'agrégateur (48h) est écoulé, d'en-attente vers disponible. Voir
	// wallet.WalletUseCase.BasculerFondsEcheus.
	jobCtx, cancelJob := context.WithCancel(context.Background())
	defer cancelJob()
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				n, err := walletUseCase.BasculerFondsEcheus(jobCtx)
				if err != nil {
					logger.Error().Err(err).Msg("bascule des fonds échus")
					continue
				}
				if n > 0 {
					logger.Info().Int("nombre", n).Msg("fonds basculés d'en-attente vers disponible")
				}
			}
		}
	}()

	// Job planifié : détecte les dépenses sur les cartes actives par
	// rapprochement de solde, faute de webhook de transaction carte côté
	// agrégateur. Voir carte.CarteUseCase.SynchroniserSoldes. Le ticker
	// tourne au rythme le plus fin qu'on utilise (20s, la base de
	// l'escalade côté domaine) : la plupart des passages ne trouvent aucune
	// carte due (ListAVerifier filtre déjà côté DB), seul un appel externe
	// réel coûte quelque chose.
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				n, err := carteUseCase.SynchroniserSoldes(jobCtx)
				if err != nil {
					logger.Error().Err(err).Msg("synchronisation des soldes de carte")
					continue
				}
				if n > 0 {
					logger.Info().Int("nombre", n).Msg("dépenses détectées sur des cartes")
				}
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("arrêt du serveur en cours")
	cancelJob()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("erreur pendant l'arrêt")
	}
}
