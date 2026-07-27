// Package jwt implémente output.TokenGenerator avec des JSON Web Tokens
// signés HS256.
package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"raycard/internal/core/domain"
	"raycard/internal/core/ports/output"
)

const dureeAccessToken = 15 * time.Minute

// claimsJWT porte, en plus des claims standard (sujet, expiration...),
// le rôle de l'utilisateur — nécessaire pour que middleware.RequireAdmin
// puisse trancher sans relire la base à chaque requête.
type claimsJWT struct {
	jwt.RegisteredClaims
	Role domain.RoleUtilisateur `json:"role"`
}

type TokenGenerator struct {
	secret []byte
}

func NewTokenGenerator(secret string) *TokenGenerator {
	return &TokenGenerator{secret: []byte(secret)}
}

func (g *TokenGenerator) GenererAccessToken(claims output.Claims) (string, time.Time, error) {
	maintenant := time.Now().UTC()
	expireAt := maintenant.Add(dureeAccessToken)

	jwtClaims := claimsJWT{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   claims.UtilisateurID,
			IssuedAt:  jwt.NewNumericDate(maintenant),
			ExpiresAt: jwt.NewNumericDate(expireAt),
		},
		Role: claims.Role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	signe, err := token.SignedString(g.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signe, expireAt, nil
}

func (g *TokenGenerator) ValiderAccessToken(tokenString string) (output.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &claimsJWT{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return g.secret, nil
	})
	if err != nil || !token.Valid {
		return output.Claims{}, domain.ErrTokenInvalide
	}

	claims, ok := token.Claims.(*claimsJWT)
	if !ok || claims.Subject == "" {
		return output.Claims{}, domain.ErrTokenInvalide
	}
	return output.Claims{UtilisateurID: claims.Subject, Role: claims.Role}, nil
}
