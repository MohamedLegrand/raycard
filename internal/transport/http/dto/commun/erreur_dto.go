// Package commun contient les structures JSON exposées par l'API HTTP
// partagées par plusieurs modules.
package commun

// ErreurDTO est le format JSON uniforme de toute réponse d'erreur HTTP
// (voir l'ErrorHandler de cmd/api/main.go).
type ErreurDTO struct {
	Erreur string `json:"erreur" example:"un compte existe déjà avec cet email"`
}
