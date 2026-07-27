// Package jwt implémente output.TokenGenerator avec des JSON Web Tokens
// signés HS256.
package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"raycard/internal/core/domain"
)

const dureeAccessToken = 15 * time.Minute

type TokenGenerator struct {
	secret []byte
}

func NewTokenGenerator(secret string) *TokenGenerator {
	return &TokenGenerator{secret: []byte(secret)}
}

func (g *TokenGenerator) GenererAccessToken(utilisateurID string) (string, time.Time, error) {
	maintenant := time.Now().UTC()
	expireAt := maintenant.Add(dureeAccessToken)

	claims := jwt.RegisteredClaims{
		Subject:   utilisateurID,
		IssuedAt:  jwt.NewNumericDate(maintenant),
		ExpiresAt: jwt.NewNumericDate(expireAt),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signe, err := token.SignedString(g.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signe, expireAt, nil
}

func (g *TokenGenerator) ValiderAccessToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return g.secret, nil
	})
	if err != nil || !token.Valid {
		return "", domain.ErrTokenInvalide
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return "", domain.ErrTokenInvalide
	}
	return claims.Subject, nil
}
