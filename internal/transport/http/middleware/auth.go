package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"raycard/internal/core/domain"
	"raycard/internal/core/ports/output"
)

// Clés utilisées pour stocker les informations de l'utilisateur
// authentifié dans les locals Fiber (voir handlers protégés en aval).
const (
	CleContextUtilisateurID = "utilisateur_id"
	CleContextRole          = "role"
)

// RequireAuth vérifie l'en-tête "Authorization: Bearer <token>" et
// injecte l'identifiant utilisateur et son rôle dans le contexte de la
// requête. Toute route enregistrée après ce middleware est protégée.
func RequireAuth(tokenGenerator output.TokenGenerator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := validerEntete(c, tokenGenerator)
		if err != nil {
			return err
		}

		c.Locals(CleContextUtilisateurID, claims.UtilisateurID)
		c.Locals(CleContextRole, claims.Role)
		return c.Next()
	}
}

// RequireAdmin est équivalent à RequireAuth, en exigeant en plus que
// l'utilisateur ait le rôle admin. Utilisé pour toutes les routes
// back-office.
func RequireAdmin(tokenGenerator output.TokenGenerator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := validerEntete(c, tokenGenerator)
		if err != nil {
			return err
		}
		if claims.Role != domain.RoleAdmin {
			return fiber.NewError(fiber.StatusForbidden, "accès réservé aux administrateurs")
		}

		c.Locals(CleContextUtilisateurID, claims.UtilisateurID)
		c.Locals(CleContextRole, claims.Role)
		return c.Next()
	}
}

func validerEntete(c *fiber.Ctx, tokenGenerator output.TokenGenerator) (output.Claims, error) {
	entete := c.Get(fiber.HeaderAuthorization)
	const prefixe = "Bearer "
	if !strings.HasPrefix(entete, prefixe) {
		return output.Claims{}, fiber.NewError(fiber.StatusUnauthorized, "en-tête Authorization manquant ou invalide")
	}

	token := strings.TrimPrefix(entete, prefixe)
	claims, err := tokenGenerator.ValiderAccessToken(token)
	if err != nil {
		return output.Claims{}, fiber.NewError(fiber.StatusUnauthorized, "access token invalide ou expiré")
	}
	return claims, nil
}
