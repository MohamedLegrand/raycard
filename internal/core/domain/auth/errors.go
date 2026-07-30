package auth

import "errors"

// Erreurs spécifiques à l'authentification.
var (
	ErrIdentifiantsInvalides = errors.New("email ou mot de passe invalide")
	ErrTokenInvalide         = errors.New("token invalide, expiré ou révoqué")
)
