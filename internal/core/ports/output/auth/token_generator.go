package auth

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// Claims regroupe les informations portées par un access token.
type Claims struct {
	UtilisateurID string
	Role          commun.RoleUtilisateur
}

// TokenGenerator signe et valide les access tokens (JWT). Le refresh
// token, lui, est un secret opaque généré et haché par l'application
// (voir application/auth/auth_service.go) : il ne transite pas par ce port.
type TokenGenerator interface {
	// GenererAccessToken signe un access token portant les claims
	// données et renvoie sa date d'expiration.
	GenererAccessToken(claims Claims) (token string, expireAt time.Time, err error)

	// ValiderAccessToken vérifie la signature et l'expiration du token et
	// renvoie les claims qu'il porte.
	ValiderAccessToken(token string) (Claims, error)
}
