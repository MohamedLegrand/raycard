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

	ErrVerrouReinitialisationIntrouvable = errors.New("verrou de réinitialisation introuvable")
	// ErrTropDeTentativesReinitialisation protège /reinitialiser-mot-de-passe
	// contre le bourrage du code à 6 chiffres : contrairement au ticket de
	// 2FA (voir TicketConnexion), ce code n'a pas de verrou par tentative
	// intrinsèque puisque la requête ne porte aucun identifiant de
	// "cible" — seul un verrou par IP (voir VerrouReinitialisation) peut
	// combler ce manque.
	ErrTropDeTentativesReinitialisation = errors.New("trop de tentatives de réinitialisation, réessayez plus tard")
)
