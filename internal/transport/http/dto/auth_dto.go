package dto

import (
	"time"

	"raycard/internal/core/ports/input"
)

type ConnexionRequestDTO struct {
	Email      string `json:"email" validate:"required,email" example:"awa.kone@example.com"`
	MotDePasse string `json:"mot_de_passe" validate:"required" example:"motdepasse123"`
}

func (d ConnexionRequestDTO) ToUseCaseRequest() input.ConnexionRequest {
	return input.ConnexionRequest{Email: d.Email, MotDePasse: d.MotDePasse}
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

func FromSessionResultat(res *input.SessionResultat) SessionResponseDTO {
	return SessionResponseDTO{
		AccessToken:          res.AccessToken,
		AccessTokenExpireAt:  res.AccessTokenExpireAt,
		RefreshToken:         res.RefreshToken,
		RefreshTokenExpireAt: res.RefreshTokenExpireAt,
	}
}
