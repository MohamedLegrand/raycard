// Package harnais fournit l'infrastructure commune aux tests HTTP de la
// couche transport (voir test/transport/http/carte, et les paquets
// équivalents à venir pour les autres modules) : construction d'un
// fiber.App réel câblé via apihttp.SetupRoutes, avec les mêmes middlewares
// qu'en production (apihttp.NouvelleApp, un vrai jwt.TokenGenerator —
// jamais fake, la validation de signature/expiration fait partie du
// comportement vérifié ici) mais des use cases FAKES en entrée. Ces tests
// couvrent le routing, la validation des DTO et le mapping d'erreurs —
// jamais la base de données, hors de portée ici (voir l'audit backend qui
// a motivé ce paquet : la couche transport n'était jusque-là vérifiée que
// manuellement).
package harnais

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain/commun"
	authoutput "raycard/internal/core/ports/output/auth"
	"raycard/internal/infrastructure/auth/jwt"
	apihttp "raycard/internal/transport/http"
)

// SecretTest respecte la même contrainte de longueur que
// config.validerJWTSecret en production (32 caractères minimum), même si
// config.Load n'est jamais appelé dans ces tests — cohérence avec ce
// qu'un vrai déploiement exigerait.
const SecretTest = "secret-de-test-jwt-32-caracteres-minimum"

// NouveauTokenGenerator construit un vrai générateur JWT (jamais fake :
// c'est justement le middleware d'authentification, avec sa vraie
// vérification de signature/expiration, que ces tests veulent exercer).
func NouveauTokenGenerator() authoutput.TokenGenerator {
	return jwt.NewTokenGenerator(SecretTest)
}

// Token génère un access token valide portant les claims données.
func Token(t *testing.T, tg authoutput.TokenGenerator, utilisateurID string, role commun.RoleUtilisateur) string {
	t.Helper()
	token, _, err := tg.GenererAccessToken(authoutput.Claims{UtilisateurID: utilisateurID, Role: role})
	require.NoError(t, err)
	return token
}

// NouvelleApp construit l'application avec toutes les routes de
// apihttp.SetupRoutes. Seuls les champs non-nil de h correspondent à un
// handler réellement exerçable : former un method value sur un champ nil
// (ex: h.Auth si le test ne porte que sur les cartes) est sans danger en
// Go tant que la méthode n'est jamais appelée — n'envoyer de requête de
// test que vers les routes couvertes par les handlers fournis.
func NouvelleApp(h apihttp.Handlers) (*fiber.App, authoutput.TokenGenerator) {
	tokenGenerator := NouveauTokenGenerator()
	app := apihttp.NouvelleApp()
	apihttp.SetupRoutes(app, h, tokenGenerator)
	return app, tokenGenerator
}
