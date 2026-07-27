package output

import "time"

// TokenGenerator signe et valide les access tokens (JWT). Le refresh
// token, lui, est un secret opaque généré et haché par l'application
// (voir application/auth_service.go) : il ne transite pas par ce port.
type TokenGenerator interface {
	// GenererAccessToken signe un access token pour l'utilisateur donné
	// et renvoie sa date d'expiration.
	GenererAccessToken(utilisateurID string) (token string, expireAt time.Time, err error)

	// ValiderAccessToken vérifie la signature et l'expiration du token et
	// renvoie l'identifiant utilisateur porté par ses claims.
	ValiderAccessToken(token string) (utilisateurID string, err error)
}
