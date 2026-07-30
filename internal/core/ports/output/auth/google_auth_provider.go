package auth

import "context"

// IdentiteGoogle regroupe les informations extraites d'un ID token
// Google valide. EmailVerifie indique si Google a lui-même confirmé la
// propriété de cet email — condition nécessaire avant de lier ce compte
// Google à un utilisateur RAYCARD existant (voir AuthUseCase.ConnexionGoogle).
type IdentiteGoogle struct {
	GoogleID     string
	Email        string
	EmailVerifie bool
	Nom          string
	Prenom       string
}

// GoogleAuthProvider vérifie un ID token émis par Google (signature,
// expiration, audience) et en extrait l'identité portée. Isole le reste
// du code de la bibliothèque Google sous-jacente.
type GoogleAuthProvider interface {
	VerifierIDToken(ctx context.Context, idToken string) (IdentiteGoogle, error)
}
