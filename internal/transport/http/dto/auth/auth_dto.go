// Package auth contient les structures JSON exposées par l'API HTTP
// pour l'authentification et leur conversion vers/depuis les types de
// la couche application.
package auth

import (
	"time"

	"raycard/internal/core/ports/input/auth"
)

type ConnexionRequestDTO struct {
	Email      string `json:"email" validate:"required,email" example:"awa.kone@example.com"`
	MotDePasse string `json:"mot_de_passe" validate:"required" example:"motdepasse123"`
}

func (d ConnexionRequestDTO) ToUseCaseRequest() auth.ConnexionRequest {
	return auth.ConnexionRequest{Email: d.Email, MotDePasse: d.MotDePasse}
}

// ConnexionResponseDTO est la réponse de POST /auth/connexion : la 2FA
// étant obligatoire, aucun token de session n'est encore délivré ici —
// juste un ticket à présenter avec le code reçu par email.
type ConnexionResponseDTO struct {
	Ticket        string `json:"ticket"`
	ExpireDansSec int    `json:"expire_dans_sec"`
}

func FromConnexionResultat(res *auth.ConnexionResultat) ConnexionResponseDTO {
	return ConnexionResponseDTO{Ticket: res.Ticket, ExpireDansSec: res.ExpireDansSec}
}

type VerifierCode2FARequestDTO struct {
	Ticket string `json:"ticket" validate:"required"`
	Code   string `json:"code" validate:"required,len=6,numeric" example:"042951"`
}

// ConnexionGoogleRequestDTO : Telephone/PaysCode ne sont utilisés que
// si aucun compte n'existe encore pour cet utilisateur (création à la
// volée) — ignorés sinon, mais toujours exigés dans le corps de la
// requête pour garder un contrat simple et unique.
type ConnexionGoogleRequestDTO struct {
	IDToken   string `json:"id_token" validate:"required"`
	Telephone string `json:"telephone" validate:"required" example:"+2250700000000"`
	PaysCode  string `json:"pays_code" validate:"required,len=2" example:"CI"`
}

func (d ConnexionGoogleRequestDTO) ToUseCaseRequest() auth.ConnexionGoogleRequest {
	return auth.ConnexionGoogleRequest{IDToken: d.IDToken, Telephone: d.Telephone, PaysCode: d.PaysCode}
}

type RafraichirRequestDTO struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type SessionResponseDTO struct {
	AccessToken          string    `json:"access_token"`
	AccessTokenExpireAt  time.Time `json:"access_token_expire_at"`
	RefreshToken         string    `json:"refresh_token"`
	RefreshTokenExpireAt time.Time `json:"refresh_token_expire_at"`
}

func FromSessionResultat(res *auth.SessionResultat) SessionResponseDTO {
	return SessionResponseDTO{
		AccessToken:          res.AccessToken,
		AccessTokenExpireAt:  res.AccessTokenExpireAt,
		RefreshToken:         res.RefreshToken,
		RefreshTokenExpireAt: res.RefreshTokenExpireAt,
	}
}

type DemanderReinitialisationRequestDTO struct {
	Email string `json:"email" validate:"required,email" example:"awa.kone@example.com"`
}

type ReinitialiserRequestDTO struct {
	Token             string `json:"token" validate:"required,len=6,numeric" example:"042951"`
	NouveauMotDePasse string `json:"nouveau_mot_de_passe" validate:"required,min=8" example:"nouveaumotdepasse123"`
}
