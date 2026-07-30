// Package auth regroupe les interfaces (ports entrants) exposées par la
// couche application au transport pour l'authentification et le cycle
// de vie de la session.
package auth

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

// ConnexionResultat est émis après vérification du mot de passe, mais
// avant l'obtention de la session : la 2FA étant obligatoire, aucun
// access/refresh token n'est encore délivré à ce stade. Ticket doit être
// présenté avec le code reçu par email pour obtenir une SessionResultat
// (voir AuthUseCase.VerifierCode2FA).
type ConnexionResultat struct {
	Ticket        string
	ExpireDansSec int
}

// ConnexionGoogleRequest transporte l'ID token émis par Google, ainsi
// que téléphone/pays — nécessaires seulement si aucun compte n'existe
// encore pour cet utilisateur (création à la volée), ignorés sinon.
type ConnexionGoogleRequest struct {
	IDToken   string
	Telephone string
	PaysCode  string
}

// MetadonneesConnexion capture le contexte de la requête au moment de
// la vérification du second facteur, pour les notifications de
// sécurité (email de nouvelle connexion). Purement informatif, jamais
// utilisé pour une décision de sécurité.
type MetadonneesConnexion struct {
	AdresseIP    string
	AppareilInfo string
}

// AuthUseCase orchestre l'authentification et le cycle de vie de la
// session (access + refresh token, rotation, révocation).
type AuthUseCase interface {
	// Connexion vérifie les identifiants et déclenche le second facteur
	// (envoi d'un code par email) : elle ne renvoie jamais directement de
	// session, la 2FA est obligatoire pour tout le monde.
	Connexion(ctx context.Context, req ConnexionRequest) (*ConnexionResultat, error)

	// VerifierCode2FA échange un ticket de connexion valide et son code
	// contre une session complète (access + refresh token). Envoie une
	// notification de connexion réussie, et une alerte si les tentatives
	// s'épuisent (voir application/auth/auth_service.go).
	VerifierCode2FA(ctx context.Context, ticket, code string, metadonnees MetadonneesConnexion) (*SessionResultat, error)

	// ConnexionGoogle authentifie (ou crée) un utilisateur à partir d'un
	// ID token Google déjà vérifié côté client, puis déclenche le même
	// second facteur que Connexion : ne renvoie jamais directement de
	// session, la 2FA est obligatoire pour tout le monde, y compris via
	// Google.
	ConnexionGoogle(ctx context.Context, req ConnexionGoogleRequest) (*ConnexionResultat, error)

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
