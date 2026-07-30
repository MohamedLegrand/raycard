// Package auth regroupe les ports sortants propres à l'authentification :
// sessions (refresh tokens), réinitialisation de mot de passe, 2FA,
// génération de tokens, Google Sign-In et notifications. Les
// implémentations concrètes vivent dans internal/infrastructure.
package auth

import (
	"context"

	"raycard/internal/core/domain/auth"
)

// RefreshTokenRepository persiste les refresh tokens (hashés). Une
// absence de résultat doit être signalée par auth.ErrTokenInvalide.
type RefreshTokenRepository interface {
	Create(ctx context.Context, rt *auth.RefreshToken) error
	FindByHash(ctx context.Context, tokenHash string) (*auth.RefreshToken, error)
	Revoke(ctx context.Context, id string) error

	// RevokeAllForUtilisateur révoque toutes les sessions actives d'un
	// utilisateur (ex : après une réinitialisation de mot de passe). Ne
	// renvoie pas d'erreur si l'utilisateur n'avait aucune session active.
	RevokeAllForUtilisateur(ctx context.Context, utilisateurID string) error
}

// TokenReinitialisationRepository persiste les codes de réinitialisation
// de mot de passe (hashés). Une absence de résultat doit être signalée
// par auth.ErrTokenInvalide.
type TokenReinitialisationRepository interface {
	Create(ctx context.Context, t *auth.TokenReinitialisation) error
	FindByHash(ctx context.Context, tokenHash string) (*auth.TokenReinitialisation, error)
	MarquerUtilise(ctx context.Context, id string) error
}

// TicketConnexionRepository persiste les connexions en attente de
// second facteur (2FA). Une absence de résultat doit être signalée par
// auth.ErrTokenInvalide.
type TicketConnexionRepository interface {
	Create(ctx context.Context, t *auth.TicketConnexion) error
	FindByHash(ctx context.Context, ticketHash string) (*auth.TicketConnexion, error)
	Update(ctx context.Context, t *auth.TicketConnexion) error
}
