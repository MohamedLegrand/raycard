package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"raycard/internal/core/ports/output"
)

// CleContextUtilisateurID est la clé utilisée pour stocker l'identifiant
// de l'utilisateur authentifié dans les locals Fiber (voir handlers
// protégés en aval, ex: back-office KYC).
const CleContextUtilisateurID = "utilisateur_id"

// RequireAuth vérifie l'en-tête "Authorization: Bearer <token>" et
// injecte l'identifiant utilisateur authentifié dans le contexte de la
// requête. Toute route enregistrée après ce middleware est protégée.
func RequireAuth(tokenGenerator output.TokenGenerator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		entete := c.Get(fiber.HeaderAuthorization)
		const prefixe = "Bearer "
		if !strings.HasPrefix(entete, prefixe) {
			return fiber.NewError(fiber.StatusUnauthorized, "en-tête Authorization manquant ou invalide")
		}

		token := strings.TrimPrefix(entete, prefixe)
		utilisateurID, err := tokenGenerator.ValiderAccessToken(token)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "access token invalide ou expiré")
		}

		c.Locals(CleContextUtilisateurID, utilisateurID)
		return c.Next()
	}
}
