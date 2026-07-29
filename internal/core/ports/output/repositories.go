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
	FindByGoogleID(ctx context.Context, googleID string) (*domain.Utilisateur, error)
	UpdateStatutKyc(ctx context.Context, u *domain.Utilisateur) error
	UpdateMotDePasse(ctx context.Context, u *domain.Utilisateur) error
	LierGoogleID(ctx context.Context, u *domain.Utilisateur) error
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

// RefreshTokenRepository persiste les refresh tokens (hashés). Une
// absence de résultat doit être signalée par domain.ErrTokenInvalide.
type RefreshTokenRepository interface {
	Create(ctx context.Context, rt *domain.RefreshToken) error
	FindByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	Revoke(ctx context.Context, id string) error

	// RevokeAllForUtilisateur révoque toutes les sessions actives d'un
	// utilisateur (ex : après une réinitialisation de mot de passe). Ne
	// renvoie pas d'erreur si l'utilisateur n'avait aucune session active.
	RevokeAllForUtilisateur(ctx context.Context, utilisateurID string) error
}

// TokenReinitialisationRepository persiste les codes de réinitialisation
// de mot de passe (hashés). Une absence de résultat doit être signalée
// par domain.ErrTokenInvalide.
type TokenReinitialisationRepository interface {
	Create(ctx context.Context, t *domain.TokenReinitialisation) error
	FindByHash(ctx context.Context, tokenHash string) (*domain.TokenReinitialisation, error)
	MarquerUtilise(ctx context.Context, id string) error
}

// TicketConnexionRepository persiste les connexions en attente de
// second facteur (2FA). Une absence de résultat doit être signalée par
// domain.ErrTokenInvalide.
type TicketConnexionRepository interface {
	Create(ctx context.Context, t *domain.TicketConnexion) error
	FindByHash(ctx context.Context, ticketHash string) (*domain.TicketConnexion, error)
	Update(ctx context.Context, t *domain.TicketConnexion) error
}

// DossierKycRepository persiste les demandes de passage de palier KYC.
type DossierKycRepository interface {
	Create(ctx context.Context, d *domain.DossierKyc) error
	FindByID(ctx context.Context, id string) (*domain.DossierKyc, error)
	FindEnAttenteByUtilisateurID(ctx context.Context, utilisateurID string) (*domain.DossierKyc, error)
	ListEnAttente(ctx context.Context) ([]*domain.DossierKyc, error)
	Update(ctx context.Context, d *domain.DossierKyc) error
}

// AuditLogRepository persiste la trace des actions administrateur
// sensibles, séparément des logs applicatifs.
type AuditLogRepository interface {
	Create(ctx context.Context, entry *domain.AuditLog) error
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
