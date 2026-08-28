// Package auth fournit les middlewares Fiber d'authentification et
// d'autorisation (RBAC) partagés par toutes les routes protégées.
package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"raycard/internal/core/domain/commun"
	authoutput "raycard/internal/core/ports/output/auth"
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
func RequireAuth(tokenGenerator authoutput.TokenGenerator) fiber.Handler {
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
// l'utilisateur ait le rôle admin OU super_admin — les deux ont les
// mêmes droits sur tout le back-office (utilisateurs, wallets, cartes,
// KYC). Utilisé pour toutes les routes back-office.
func RequireAdmin(tokenGenerator authoutput.TokenGenerator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := validerEntete(c, tokenGenerator)
		if err != nil {
			return err
		}
		if claims.Role != commun.RoleAdmin && claims.Role != commun.RoleSuperAdmin {
			return fiber.NewError(fiber.StatusForbidden, "accès réservé aux administrateurs")
		}

		c.Locals(CleContextUtilisateurID, claims.UtilisateurID)
		c.Locals(CleContextRole, claims.Role)
		return c.Next()
	}
}

// RequireSuperAdmin est équivalent à RequireAdmin, en exigeant en plus
// le rôle super_admin précisément — réservé aux routes qui gèrent les
// autres administrateurs (changer un rôle est une élévation de
// privilège, ça ne doit pas être à la portée d'un admin ordinaire).
func RequireSuperAdmin(tokenGenerator authoutput.TokenGenerator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := validerEntete(c, tokenGenerator)
		if err != nil {
			return err
		}
		if claims.Role != commun.RoleSuperAdmin {
			return fiber.NewError(fiber.StatusForbidden, "accès réservé aux super-administrateurs")
		}

		c.Locals(CleContextUtilisateurID, claims.UtilisateurID)
		c.Locals(CleContextRole, claims.Role)
		return c.Next()
	}
}

func validerEntete(c *fiber.Ctx, tokenGenerator authoutput.TokenGenerator) (authoutput.Claims, error) {
	entete := c.Get(fiber.HeaderAuthorization)
	const prefixe = "Bearer "
	if !strings.HasPrefix(entete, prefixe) {
		return authoutput.Claims{}, fiber.NewError(fiber.StatusUnauthorized, "en-tête Authorization manquant ou invalide")
	}

	token := strings.TrimPrefix(entete, prefixe)
	claims, err := tokenGenerator.ValiderAccessToken(token)
	if err != nil {
		return authoutput.Claims{}, fiber.NewError(fiber.StatusUnauthorized, "access token invalide ou expiré")
	}
	return claims, nil
}
