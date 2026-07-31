package auth

import "errors"

// Erreurs spécifiques à l'authentification.
var (
	ErrIdentifiantsInvalides  = errors.New("identifiants invalides")
	ErrTokenInvalide          = errors.New("token invalide, expiré ou révoqué")
	ErrCleAppareilIntrouvable = errors.New("clé d'appareil introuvable")
	ErrChallengeIntrouvable   = errors.New("challenge introuvable")
	ErrVerrouIntrouvable      = errors.New("verrou de connexion introuvable")
	ErrCompteVerrouille       = errors.New("compte temporairement verrouillé suite à trop de tentatives de connexion échouées")
)
