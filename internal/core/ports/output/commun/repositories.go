// Package commun regroupe les ports sortants partagés par plusieurs
// modules : persistance de l'utilisateur et du wallet, règles KYC par
// pays, audit trail, gestion des transactions. Les implémentations
// concrètes vivent dans internal/infrastructure.
package commun

import (
	"context"

	"raycard/internal/core/domain/commun"
)

// UtilisateurRepository persiste les utilisateurs. Une absence de
// résultat doit être signalée par commun.ErrUtilisateurIntrouvable,
// jamais par (nil, nil).
type UtilisateurRepository interface {
	Create(ctx context.Context, u *commun.Utilisateur) error
	FindByID(ctx context.Context, id string) (*commun.Utilisateur, error)
	FindByEmail(ctx context.Context, email string) (*commun.Utilisateur, error)
	FindByTelephone(ctx context.Context, telephone string) (*commun.Utilisateur, error)
	FindByGoogleID(ctx context.Context, googleID string) (*commun.Utilisateur, error)
	UpdateStatutKyc(ctx context.Context, u *commun.Utilisateur) error
	UpdateMotDePasse(ctx context.Context, u *commun.Utilisateur) error
	LierGoogleID(ctx context.Context, u *commun.Utilisateur) error

	// UpdateProfil met à jour les informations de profil auto-gérées par
	// l'utilisateur (nom, prénom, photo) — jamais l'email, qui suit son
	// propre circuit de vérification (voir UpdateEmail).
	UpdateProfil(ctx context.Context, u *commun.Utilisateur) error

	// UpdateEmail change l'adresse email, une fois sa propriété vérifiée.
	UpdateEmail(ctx context.Context, u *commun.Utilisateur) error

	// ListAll liste les utilisateurs, pour le back-office — jamais utilisé
	// par un flux client. Les plus récents d'abord.
	ListAll(ctx context.Context, filtre FiltreUtilisateurs) ([]*commun.Utilisateur, error)
}

// FiltreUtilisateurs restreint le listing back-office des utilisateurs ;
// un champ vide ne filtre pas dessus. Recherche s'applique par
// correspondance partielle sur l'email ou le téléphone.
type FiltreUtilisateurs struct {
	Recherche string
}

// WalletRepository persiste les wallets. V1 : un wallet par utilisateur.
type WalletRepository interface {
	Create(ctx context.Context, w *commun.Wallet) error
	FindByID(ctx context.Context, id string) (*commun.Wallet, error)
	FindByUtilisateurID(ctx context.Context, utilisateurID string) (*commun.Wallet, error)
	UpdateSolde(ctx context.Context, w *commun.Wallet) error
}

// ReglesKycRepository lit les plafonds par pays/palier depuis
// regles_kyc_pays. Ne renvoie jamais de plafond codé en dur.
type ReglesKycRepository interface {
	FindByPaysEtTier(ctx context.Context, paysCode string, tier commun.KycTier) (*commun.RegleKyc, error)
}

// AuditLogRepository persiste la trace des actions administrateur
// sensibles, séparément des logs applicatifs.
type AuditLogRepository interface {
	Create(ctx context.Context, entry *commun.AuditLog) error

	// List consulte l'historique d'audit, pour le back-office. Les plus
	// récentes d'abord.
	List(ctx context.Context, filtre FiltreAuditLog) ([]*commun.AuditLog, error)
}

// FiltreAuditLog restreint le listing des entrées d'audit ; un champ
// vide ne filtre pas dessus.
type FiltreAuditLog struct {
	AdminID   string
	CibleType string
	CibleID   string
}

// StockageFichier persiste le contenu brut d'un fichier téléversé (ex:
// document d'identité, photo de profil) et permet de le relire ensuite
// (ex: pour l'OCR, ou pour l'affichage côté back-office).
type StockageFichier interface {
	Sauvegarder(ctx context.Context, nomFichier string, contenu []byte) (chemin string, err error)
	Lire(ctx context.Context, chemin string) (contenu []byte, err error)
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
