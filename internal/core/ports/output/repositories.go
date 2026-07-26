// Package output regroupe les interfaces (ports sortants) que le
// domaine et l'application utilisent pour parler au monde extérieur :
// persistance, transactions. Les implémentations concrètes vivent dans
// internal/infrastructure.
package output

import (
	"context"

	"raycard/internal/core/domain"
)

// UtilisateurRepository persiste les utilisateurs. Une absence de
// résultat doit être signalée par domain.ErrUtilisateurIntrouvable,
// jamais par (nil, nil).
type UtilisateurRepository interface {
	Create(ctx context.Context, u *domain.Utilisateur) error
	FindByID(ctx context.Context, id string) (*domain.Utilisateur, error)
	FindByEmail(ctx context.Context, email string) (*domain.Utilisateur, error)
	FindByTelephone(ctx context.Context, telephone string) (*domain.Utilisateur, error)
	UpdateStatutKyc(ctx context.Context, u *domain.Utilisateur) error
}

// WalletRepository persiste les wallets. V1 : un wallet par utilisateur.
type WalletRepository interface {
	Create(ctx context.Context, w *domain.Wallet) error
	FindByID(ctx context.Context, id string) (*domain.Wallet, error)
	FindByUtilisateurID(ctx context.Context, utilisateurID string) (*domain.Wallet, error)
	UpdateSolde(ctx context.Context, w *domain.Wallet) error
}

// ReglesKycRepository lit les plafonds par pays/palier depuis
// regles_kyc_pays. Ne renvoie jamais de plafond codé en dur.
type ReglesKycRepository interface {
	FindByPaysEtTier(ctx context.Context, paysCode string, tier domain.KycTier) (*domain.RegleKyc, error)
}

// TxManager exécute fn dans une transaction ACID unique. L'implémentation
// place la transaction dans le context.Context transmis à fn ; les
// repositories doivent la récupérer pour que toutes leurs opérations
// participent à la même transaction.
//
// Utilisé par tout flux touchant plusieurs tables à la fois (ex :
// inscription = création utilisateur + création wallet).
type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
