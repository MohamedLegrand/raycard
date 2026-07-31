// Package http câble les routes Fiber vers les handlers. Importé sous
// alias (ex: apihttp) par cmd/api/main.go pour éviter toute confusion
// avec net/http.
package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"

	authoutput "raycard/internal/core/ports/output/auth"
	handlersauth "raycard/internal/transport/http/handlers/auth"
	handlerskyc "raycard/internal/transport/http/handlers/kyc"
	authmw "raycard/internal/transport/http/middleware/auth"
)

// Handlers regroupe tous les handlers HTTP câblés par main.go. Un
// champ est ajouté ici à chaque nouveau module (wallet, cartes...).
type Handlers struct {
	Kyc      *handlerskyc.KycHandler
	Auth     *handlersauth.AuthHandler
	AdminKyc *handlerskyc.AdminKycHandler
}

func SetupRoutes(app *fiber.App, h Handlers, tokenGenerator authoutput.TokenGenerator) {
	// Doc générée par `swag init` (voir Makefile / docs/), servie hors du
	// groupe /api/v1 puisque ce n'est pas un endpoint métier.
	app.Get("/swagger/*", swagger.HandlerDefault)

	api := app.Group("/api/v1")

	kyc := api.Group("/kyc")
	kyc.Post("/inscription", h.Kyc.Inscrire)
	kyc.Post("/demande-tier2", authmw.RequireAuth(tokenGenerator), h.Kyc.DemanderTier2)
	kyc.Post("/documents", authmw.RequireAuth(tokenGenerator), h.Kyc.TeleverserDocument)

	auth := api.Group("/auth")
	auth.Post("/connexion", h.Auth.Connexion)
	auth.Post("/connexion/verifier-code", h.Auth.VerifierCode2FA)
	auth.Post("/connexion-google", h.Auth.ConnexionGoogle)
	auth.Post("/rafraichir", h.Auth.Rafraichir)
	auth.Post("/deconnexion", h.Auth.Deconnexion)
	auth.Post("/mot-de-passe-oublie", h.Auth.DemanderReinitialisation)
	auth.Post("/reinitialiser-mot-de-passe", h.Auth.Reinitialiser)

	auth.Post("/empreinte/appareils", authmw.RequireAuth(tokenGenerator), h.Auth.EnregistrerAppareil)
	auth.Delete("/empreinte/appareils/:id", authmw.RequireAuth(tokenGenerator), h.Auth.RevoquerAppareil)
	auth.Post("/empreinte/challenge", h.Auth.DemanderChallengeEmpreinte)
	auth.Post("/empreinte/verifier", h.Auth.ConnexionEmpreinte)

	auth.Put("/profil", authmw.RequireAuth(tokenGenerator), h.Auth.ModifierProfil)
	auth.Post("/profil/photo", authmw.RequireAuth(tokenGenerator), h.Auth.ModifierPhotoProfil)
	auth.Post("/profil/mot-de-passe", authmw.RequireAuth(tokenGenerator), h.Auth.ChangerMotDePasse)
	auth.Post("/profil/email", authmw.RequireAuth(tokenGenerator), h.Auth.DemanderChangementEmail)
	auth.Post("/profil/email/confirmer", h.Auth.ConfirmerChangementEmail)

	backofficeKyc := api.Group("/backoffice/kyc", authmw.RequireAdmin(tokenGenerator))
	backofficeKyc.Get("/dossiers", h.AdminKyc.ListerDossiersEnAttente)
	backofficeKyc.Post("/dossiers/:id/approuver", h.AdminKyc.Approuver)
	backofficeKyc.Post("/dossiers/:id/rejeter", h.AdminKyc.Rejeter)
	backofficeKyc.Get("/utilisateurs/:id/documents", h.AdminKyc.ListerDocuments)
}
