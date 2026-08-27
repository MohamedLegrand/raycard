// Package http câble les routes Fiber vers les handlers. Importé sous
// alias (ex: apihttp) par cmd/api/main.go pour éviter toute confusion
// avec net/http.
package http

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/swagger"

	authoutput "raycard/internal/core/ports/output/auth"
	handlersadmin "raycard/internal/transport/http/handlers/admin"
	handlersauth "raycard/internal/transport/http/handlers/auth"
	handlerscarte "raycard/internal/transport/http/handlers/carte"
	handlerskyc "raycard/internal/transport/http/handlers/kyc"
	handlerswallet "raycard/internal/transport/http/handlers/wallet"
	authmw "raycard/internal/transport/http/middleware/auth"
)

// limiteurConnexion : première barrière (par IP) contre le bourrage de
// mot de passe, avant même que la protection par compte (voir
// application/auth.VerrouConnexion) n'entre en jeu. Volontairement plus
// permissif que le seuil par compte (5 échecs) : il doit surtout freiner
// les scripts qui tapent vite, pas gêner un usage normal.
var limiteurConnexion = limiter.New(limiter.Config{
	Max:        20,
	Expiration: time.Minute,
})

// limiteurInscription : freine le bourrage sur /auth/inscription, dont la
// réponse (409 si l'email existe déjà) permet sinon d'énumérer les
// comptes existants sans aucune limite. Plus strict que limiteurConnexion
// (un utilisateur légitime ne s'inscrit qu'une fois, contrairement à une
// connexion répétée).
var limiteurInscription = limiter.New(limiter.Config{
	Max:        5,
	Expiration: time.Minute,
})

// limiteurOperationWallet freine le bourrage sur les opérations qui
// déplacent réellement des fonds (topup, cashout) — contrairement aux
// routes de connexion, elles étaient jusqu'ici protégées par
// l'authentification seule, sans aucune limite de fréquence. Sert aussi
// à éviter qu'un seul utilisateur monopolise le débit partagé imposé à
// l'agrégateur (voir hrpay.delaiMinEntreAppels, 200ms entre TOUS les
// appels agrégateur, tous utilisateurs confondus).
var limiteurOperationWallet = limiter.New(limiter.Config{
	Max:        10,
	Expiration: time.Minute,
})

// Handlers regroupe tous les handlers HTTP câblés par main.go. Un
// champ est ajouté ici à chaque nouveau module (wallet, cartes...).
type Handlers struct {
	Kyc         *handlerskyc.KycHandler
	Auth        *handlersauth.AuthHandler
	AdminKyc    *handlerskyc.AdminKycHandler
	Wallet      *handlerswallet.WalletHandler
	Carte       *handlerscarte.CarteHandler
	Admin       *handlersadmin.AdminHandler
	AdminWallet *handlerswallet.AdminWalletHandler
	AdminCarte  *handlerscarte.AdminCarteHandler
}

func SetupRoutes(app *fiber.App, h Handlers, tokenGenerator authoutput.TokenGenerator) {
	// Doc générée par `swag init` (voir Makefile / docs/), servie hors du
	// groupe /api/v1 puisque ce n'est pas un endpoint métier.
	app.Get("/swagger/*", swagger.HandlerDefault)

	api := app.Group("/api/v1")

	kyc := api.Group("/kyc")
	kyc.Post("/demande-tier2", authmw.RequireAuth(tokenGenerator), h.Kyc.DemanderTier2)
	kyc.Post("/documents", authmw.RequireAuth(tokenGenerator), h.Kyc.TeleverserDocument)

	auth := api.Group("/auth")
	// L'inscription vit ici plutôt que dans /kyc : côté client, créer un
	// compte est une action d'authentification (on s'en sert ensuite pour
	// se connecter) — le fait qu'elle valide aussi le palier KYC 1 en
	// interne est un détail d'implémentation, pas ce que l'utilisateur vit.
	auth.Post("/inscription", limiteurInscription, h.Kyc.Inscrire)
	auth.Post("/connexion", limiteurConnexion, h.Auth.Connexion)
	auth.Post("/connexion/verifier-code", h.Auth.VerifierCode2FA)
	auth.Post("/connexion-google", limiteurConnexion, h.Auth.ConnexionGoogle)
	auth.Post("/rafraichir", h.Auth.Rafraichir)
	auth.Post("/deconnexion", h.Auth.Deconnexion)
	// limiteurConnexion réutilisé ici : /mot-de-passe-oublie sans limite
	// permettrait de spammer la boîte mail d'une victime ; le token de
	// /reinitialiser-mot-de-passe est un code à 6 chiffres (1M
	// possibilités) recherché par simple égalité de hash, sans verrou
	// par tentative (contrairement à la 2FA, voir VerifierCode2FA) — un
	// rate-limit par IP reste la seule protection contre le bourrage
	// aujourd'hui.
	auth.Post("/mot-de-passe-oublie", limiteurConnexion, h.Auth.DemanderReinitialisation)
	auth.Post("/reinitialiser-mot-de-passe", limiteurConnexion, h.Auth.Reinitialiser)

	auth.Post("/empreinte/appareils", authmw.RequireAuth(tokenGenerator), h.Auth.EnregistrerAppareil)
	auth.Delete("/empreinte/appareils/:id", authmw.RequireAuth(tokenGenerator), h.Auth.RevoquerAppareil)
	auth.Post("/empreinte/challenge", h.Auth.DemanderChallengeEmpreinte)
	auth.Post("/empreinte/verifier", limiteurConnexion, h.Auth.ConnexionEmpreinte)

	auth.Get("/profil", authmw.RequireAuth(tokenGenerator), h.Auth.ObtenirProfil)
	auth.Put("/profil", authmw.RequireAuth(tokenGenerator), h.Auth.ModifierProfil)
	auth.Get("/profil/photo", authmw.RequireAuth(tokenGenerator), h.Auth.ObtenirPhotoProfil)
	auth.Post("/profil/photo", authmw.RequireAuth(tokenGenerator), h.Auth.ModifierPhotoProfil)
	auth.Post("/profil/mot-de-passe", authmw.RequireAuth(tokenGenerator), h.Auth.ChangerMotDePasse)
	auth.Post("/profil/email", authmw.RequireAuth(tokenGenerator), h.Auth.DemanderChangementEmail)
	auth.Post("/profil/email/confirmer", h.Auth.ConfirmerChangementEmail)

	api.Get("/wallet", authmw.RequireAuth(tokenGenerator), h.Wallet.ObtenirWallet)
	api.Get("/wallet/transactions", authmw.RequireAuth(tokenGenerator), h.Wallet.ListerTransactions)
	api.Post("/wallet/topup", authmw.RequireAuth(tokenGenerator), limiteurOperationWallet, h.Wallet.InitierRecharge)
	api.Post("/wallet/cashout", authmw.RequireAuth(tokenGenerator), limiteurOperationWallet, h.Wallet.InitierRetrait)
	// Non authentifiée par JWT : l'authenticité vient exclusivement de la
	// signature HMAC vérifiée par le use case (voir wallet.AgregateurPaiement).
	api.Post("/webhooks/hrpay", h.Wallet.WebhookHrPay)

	api.Post("/cartes", authmw.RequireAuth(tokenGenerator), h.Carte.CreerCarte)
	api.Get("/cartes", authmw.RequireAuth(tokenGenerator), h.Carte.ListerCartes)
	api.Get("/cartes/:id", authmw.RequireAuth(tokenGenerator), h.Carte.ObtenirCarte)
	api.Get("/cartes/:id/depenses", authmw.RequireAuth(tokenGenerator), h.Carte.ListerDepenses)
	api.Post("/cartes/:id/gel", authmw.RequireAuth(tokenGenerator), h.Carte.GelerCarte)
	api.Post("/cartes/:id/degel", authmw.RequireAuth(tokenGenerator), h.Carte.DegelerCarte)
	api.Post("/cartes/:id/topup", authmw.RequireAuth(tokenGenerator), h.Carte.RechargerCarte)
	api.Post("/cartes/:id/annuler", authmw.RequireAuth(tokenGenerator), h.Carte.AnnulerCarte)

	backofficeKyc := api.Group("/backoffice/kyc", authmw.RequireAdmin(tokenGenerator))
	backofficeKyc.Get("/dossiers", h.AdminKyc.ListerDossiersEnAttente)
	backofficeKyc.Post("/dossiers/:id/approuver", h.AdminKyc.Approuver)
	backofficeKyc.Post("/dossiers/:id/rejeter", h.AdminKyc.Rejeter)
	backofficeKyc.Get("/dossiers/:id/documents", h.AdminKyc.ListerDocuments)
	backofficeKyc.Get("/documents/:id", h.AdminKyc.RecupererDocument)

	backofficeUtilisateurs := api.Group("/backoffice/utilisateurs", authmw.RequireAdmin(tokenGenerator))
	backofficeUtilisateurs.Get("/", h.Admin.ListerUtilisateurs)
	backofficeUtilisateurs.Get("/:id", h.Admin.ObtenirUtilisateur)

	backofficeWallets := api.Group("/backoffice/wallets", authmw.RequireAdmin(tokenGenerator))
	backofficeWallets.Post("/:id/gel", h.AdminWallet.GelerWallet)
	backofficeWallets.Post("/:id/degel", h.AdminWallet.DegelerWallet)

	backofficeCartes := api.Group("/backoffice/cartes", authmw.RequireAdmin(tokenGenerator))
	backofficeCartes.Get("/", h.AdminCarte.ListerCartes)
	backofficeCartes.Post("/:id/gel", h.AdminCarte.GelerCarte)
	backofficeCartes.Post("/:id/degel", h.AdminCarte.DegelerCarte)
	backofficeCartes.Post("/:id/annuler", h.AdminCarte.AnnulerCarte)

	backofficeTransactions := api.Group("/backoffice/transactions", authmw.RequireAdmin(tokenGenerator))
	backofficeTransactions.Get("/", h.AdminWallet.ListerTransactions)

	backofficeAuditLogs := api.Group("/backoffice/audit-logs", authmw.RequireAdmin(tokenGenerator))
	backofficeAuditLogs.Get("/", h.Admin.ListerAuditLogs)
}
