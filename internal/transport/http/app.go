package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// TailleMaxCorpsRequete autorise l'upload de photos de documents KYC
// (recto/verso) et de photo de profil dans un même corps de requête —
// une simple photo de téléphone dépasse largement les quelques Ko d'un
// JSON métier ordinaire.
const TailleMaxCorpsRequete = 10 * 1024 * 1024

// NouvelleApp construit un *fiber.App avec la configuration commune à
// toute instance de l'API (limite de taille de corps, forme JSON des
// erreurs, récupération de panique) — partagée par cmd/api/main.go et
// les tests HTTP de la couche transport (voir test/transport/http/...),
// pour ne jamais laisser les réponses réellement servies diverger de
// celles vérifiées en test. Le logging de requêtes et le CORS restent
// propres à main.go (le second est conditionnel à la configuration, le
// premier n'a aucun intérêt en test).
func NouvelleApp() *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit: TailleMaxCorpsRequete,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			msgClient := err.Error()
			if code == fiber.StatusInternalServerError {
				msgClient = "erreur interne"
			}
			return c.Status(code).JSON(fiber.Map{"erreur": msgClient})
		},
	})
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))
	return app
}
