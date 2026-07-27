// Package input regroupe les interfaces (ports entrants) exposées par
// la couche application aux adaptateurs de transport (HTTP, etc.).
package input

import (
	"context"

	"raycard/internal/core/domain"
)

// InscriptionRequest transporte les données brutes d'inscription
// depuis le transport vers l'application. Ce n'est pas une entité du
// domaine : le mot de passe y est encore en clair (le hachage est fait
// par l'implémentation de KycUseCase).
type InscriptionRequest struct {
	Nom        string
	Prenom     string
	Email      string
	Telephone  string
	PaysCode   string
	MotDePasse string
}

// InscriptionResultat regroupe l'utilisateur et le wallet créés
// atomiquement lors de l'inscription.
type InscriptionResultat struct {
	Utilisateur *domain.Utilisateur
	Wallet      *domain.Wallet
}

// KycUseCase orchestre l'inscription et le cycle de vie du dossier KYC
// d'un utilisateur.
type KycUseCase interface {
	Inscrire(ctx context.Context, req InscriptionRequest) (*InscriptionResultat, error)

	// DemanderTier2 crée une demande de passage au Tier 2 pour
	// l'utilisateur authentifié donné. Toujours revue manuellement.
	DemanderTier2(ctx context.Context, utilisateurID string) (*domain.DossierKyc, error)
}
