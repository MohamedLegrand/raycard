package input

import (
	"context"
	"time"
)

// ConnexionRequest transporte les identifiants bruts depuis le transport.
type ConnexionRequest struct {
	Email      string
	MotDePasse string
}

// SessionResultat regroupe l'access token et le refresh token émis lors
// d'une connexion ou d'un rafraîchissement. RefreshToken est la valeur
// brute (à usage unique côté client) — jamais celle stockée en base.
type SessionResultat struct {
	AccessToken          string
	AccessTokenExpireAt  time.Time
	RefreshToken         string
	RefreshTokenExpireAt time.Time
}

// AuthUseCase orchestre l'authentification et le cycle de vie de la
// session (access + refresh token, rotation, révocation).
type AuthUseCase interface {
	Connexion(ctx context.Context, req ConnexionRequest) (*SessionResultat, error)
	RafraichirToken(ctx context.Context, refreshToken string) (*SessionResultat, error)
	Deconnexion(ctx context.Context, refreshToken string) error

	// DemanderReinitialisation envoie un code de réinitialisation par
	// email si un compte existe pour cet email. Ne renvoie jamais d'erreur
	// pour un email inconnu (évite l'énumération de comptes) : c'est à
	// l'appelant HTTP de toujours répondre le même succès générique.
	DemanderReinitialisation(ctx context.Context, email string) error

	// Reinitialiser change le mot de passe si le code fourni est valide,
	// et révoque toutes les sessions actives de l'utilisateur.
	Reinitialiser(ctx context.Context, token, nouveauMotDePasse string) error
}
