// Package http câble les routes Fiber vers les handlers. Importé sous
// alias (ex: apihttp) par cmd/api/main.go pour éviter toute confusion
// avec net/http.
package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"

	"raycard/internal/core/ports/output"
	"raycard/internal/transport/http/handlers"
	"raycard/internal/transport/http/middleware"
)

// Handlers regroupe tous les handlers HTTP câblés par main.go. Un
// champ est ajouté ici à chaque nouveau module (wallet, cartes...).
type Handlers struct {
	Kyc      *handlers.KycHandler
	Auth     *handlers.AuthHandler
	AdminKyc *handlers.AdminKycHandler
}

func SetupRoutes(app *fiber.App, h Handlers, tokenGenerator output.TokenGenerator) {
	// Doc générée par `swag init` (voir Makefile / docs/), servie hors du
	// groupe /api/v1 puisque ce n'est pas un endpoint métier.
	app.Get("/swagger/*", swagger.HandlerDefault)

	api := app.Group("/api/v1")

	kyc := api.Group("/kyc")
	kyc.Post("/inscription", h.Kyc.Inscrire)
	kyc.Post("/demande-tier2", middleware.RequireAuth(tokenGenerator), h.Kyc.DemanderTier2)

	auth := api.Group("/auth")
	auth.Post("/connexion", h.Auth.Connexion)
	auth.Post("/connexion/verifier-code", h.Auth.VerifierCode2FA)
	auth.Post("/rafraichir", h.Auth.Rafraichir)
	auth.Post("/deconnexion", h.Auth.Deconnexion)
	auth.Post("/mot-de-passe-oublie", h.Auth.DemanderReinitialisation)
	auth.Post("/reinitialiser-mot-de-passe", h.Auth.Reinitialiser)

	backofficeKyc := api.Group("/backoffice/kyc", middleware.RequireAdmin(tokenGenerator))
	backofficeKyc.Get("/dossiers", h.AdminKyc.ListerDossiersEnAttente)
	backofficeKyc.Post("/dossiers/:id/approuver", h.AdminKyc.Approuver)
	backofficeKyc.Post("/dossiers/:id/rejeter", h.AdminKyc.Rejeter)
}
