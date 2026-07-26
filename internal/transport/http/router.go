// Package http câble les routes Fiber vers les handlers. Importé sous
// alias (ex: apihttp) par cmd/api/main.go pour éviter toute confusion
// avec net/http.
package http

import (
	"github.com/gofiber/fiber/v2"

	"raycard/internal/transport/http/handlers"
)

// Handlers regroupe tous les handlers HTTP câblés par main.go. Un
// champ est ajouté ici à chaque nouveau module (wallet, cartes...).
type Handlers struct {
	Kyc *handlers.KycHandler
}

func SetupRoutes(app *fiber.App, h Handlers) {
	api := app.Group("/api/v1")

	kyc := api.Group("/kyc")
	kyc.Post("/inscription", h.Kyc.Inscrire)
}
