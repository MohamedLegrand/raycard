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
}
